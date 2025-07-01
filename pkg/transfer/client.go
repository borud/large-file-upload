package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client for the transfer service.
type Client struct {
	config ClientConfig
	conn   *grpc.ClientConn
	client tv1.TransferServiceClient
}

// ClientConfig is the configuration parameters for the client.
type ClientConfig struct {
	ServerAddr string
	QuitAfter  int
}

// uploadState is the upload state tracked throughout the upload and partially
// saved to disk in order to be able to resume uploads.
type uploadState struct {
	ID        string `json:"id"`
	FileSize  int64  `json:"-"`
	Offset    int64  `json:"-"`
	BlockSize int64  `json:"-"`
}

const (
	stateFileSuffix      = "upload"
	stateFilePermissions = 0600
)

// CreateClient creates a new transfer client.
func CreateClient(c ClientConfig) (*Client, error) {
	conn, err := grpc.NewClient(c.ServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{
		client: tv1.NewTransferServiceClient(conn),
		conn:   conn,
		config: c,
	}, nil
}

// Upload a file to the transfer server.
func (c *Client) Upload(filename string) (string, error) {
	state, err := c.createOrResumeUpload(filename, []byte{0})
	if err != nil {
		return "", err
	}

	in, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("error opening file [%s]: %w", filename, err)
	}
	defer in.Close()

	offset, err := in.Seek(state.Offset, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("failed to seek to correct offset: %w", err)
	}

	if offset != state.Offset {
		// this shouldn't happen unless something is really wrong
		return "", fmt.Errorf("offset mismatch, state=%d file=%d", state.Offset, offset)
	}

	// create upload stream
	stream, err := c.client.Upload(context.Background())
	if err != nil {
		return "", fmt.Errorf("error connecting to server [%s]: %w", c.config.ServerAddr, err)
	}

	buffer := make([]byte, state.BlockSize)
	for i := 0; ; i++ {
		n, err := in.Read(buffer)
		if err == io.EOF {
			break
		}

		checksum := sha256.Sum256(buffer[:n])
		err = stream.Send(&tv1.UploadRequest{
			Id:     state.ID,
			Offset: state.Offset,
			Data:   buffer[:n],
			Sha256: checksum[:],
		})
		if err != nil {
			return "", fmt.Errorf("upload failed: %w", err)
		}

		state.Offset += int64(n)

		// this is just for testing purposes
		if c.config.QuitAfter > 0 && i == c.config.QuitAfter-1 {
			slog.Info("quitting after QuitAfter blocks", "quitAfter", c.config.QuitAfter)
			return state.ID, nil
		}
		slog.Debug("->", "id", state.ID, "block", i, "offset", state.Offset)
	}

	// remove the state file since we're done uploading, but do this after we have
	// closed the stream and received the result.  If there is an error there isn't
	// anything sensible we can do about it.
	defer func() {
		stateFilename := c.stateFilename(filename)
		err := os.Remove(stateFilename)
		if err != nil {
			slog.Error("error removing state file", "stateFilename", stateFilename, "err", err)
		}
	}()

	_, err = stream.CloseAndRecv()
	if err != nil {
		return "", fmt.Errorf("error closing connection: %w", err)
	}

	return state.ID, nil
}

// Download file by id and place it in file named dstFile.  If the destination file exists
// an error is returned.
func (c *Client) Download(id ID, dstFile string) error {
	// open destination file first so we can detect if this fails before we
	// bother the server.
	out, err := os.OpenFile(dstFile, os.O_CREATE|os.O_APPEND|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to create output file [%s]: %w", dstFile, err)
	}
	defer out.Close()

	stream, err := c.client.Download(context.Background(), &tv1.DownloadRequest{
		Id:     id.String(),
		Offset: 0,
	})
	if err != nil {
		return err
	}

	for {
		res, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return err
		}

		checksum := sha256.Sum256(res.Data)
		if !bytes.Equal(checksum[:], res.Sha256) {
			return fmt.Errorf("checsum verification failed")
		}

		_, err = out.Write(res.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) createOrResumeUpload(filename string, meta []byte) (uploadState, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return uploadState{}, fmt.Errorf("file error for [%s]: %w", filename, err)
	}

	stateFilename := c.stateFilename(filename)

	_, err = os.Stat(stateFilename)

	// if err is nil the file exists
	if err == nil {
		slog.Info("resuming upload", "filename", filename)

		data, err := os.ReadFile(stateFilename)
		if err != nil {
			return uploadState{}, fmt.Errorf("unable to read state file [%s] for [%s]: %s", filename, stateFilename, err)
		}

		var state uploadState
		err = json.Unmarshal(data, &state)
		if err != nil {
			return uploadState{}, fmt.Errorf("unable to parse state file [%s]: %w", stateFilename, err)
		}

		// now we need to get the offset from the server
		slog.Info("getting offset from server")
		resp, err := c.client.GetOffset(context.Background(), &tv1.GetOffsetRequest{Id: state.ID})
		if err != nil {
			return uploadState{}, fmt.Errorf("error getting offset from server: %w", err)
		}
		slog.Info("->", "filename", filename, "offset", resp.Offset)

		return uploadState{
			ID:        state.ID,
			Offset:    resp.Offset,
			FileSize:  info.Size(),
			BlockSize: clampBlockSize(resp.PreferredBlocksize),
		}, err
	}

	// if we are here there was no existing file upload so we need to create
	// a new upload.
	slog.Info("new upload")
	resp, err := c.client.CreateUpload(context.Background(), &tv1.CreateUploadRequest{
		Size:     info.Size(),
		Metadata: meta,
	})
	if err != nil {
		return uploadState{}, fmt.Errorf("unable to create new upload: %w", err)
	}

	// we only need the ID in the saved state so to avoid future confusion
	// we explicitly only save the ID.
	err = c.saveState(uploadState{ID: resp.Id}, filename)
	if err != nil {
		return uploadState{}, err
	}

	return uploadState{
		ID:        resp.Id,
		FileSize:  info.Size(),
		Offset:    0,
		BlockSize: clampBlockSize(resp.PreferredBlocksize),
	}, nil
}

func (c *Client) saveState(state uploadState, filename string) error {
	slog.Info("saving state", "stateFilename", c.stateFilename(filename), "id", state.ID)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to serialize state to JSON: %w", err)
	}

	return os.WriteFile(c.stateFilename(filename), data, stateFilePermissions)
}

func (c *Client) stateFilename(filename string) string {
	return filename + "." + stateFileSuffix
}
