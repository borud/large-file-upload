package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client for the transfer service.
type Client struct {
	conn      *grpc.ClientConn
	addr      string
	client    tv1.TransferServiceClient
	quitAfter int
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
	stateFileSuffix = "upload"

	minBlockSize = 512 * 1024
	maxBlockSize = 1024 * 1024

	stateFilePermissions = 0600
)

// CreateClient creates a new transfer client.
func CreateClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{
		client: tv1.NewTransferServiceClient(conn),
		conn:   conn,
		addr:   addr,
	}, nil
}

// Upload a file to the transfer server.
func (c *Client) Upload(filename string) error {
	state, err := c.createOrResumeUpload(filename, []byte{0})
	if err != nil {
		return err
	}

	in, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening file [%s]: %w", filename, err)
	}
	defer in.Close()

	offset, err := in.Seek(state.Offset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to correct offset: %w", err)
	}

	if offset != state.Offset {
		// this shouldn't happen unless something is really wrong
		return fmt.Errorf("offset mismatch, state=%d file=%d", state.Offset, offset)
	}

	// create upload stream
	stream, err := c.client.Upload(context.Background())
	if err != nil {
		return fmt.Errorf("error connecting to server [%s]: %w", c.addr, err)
	}

	buffer := make([]byte, state.BlockSize)
	for i := 0; ; i++ {
		slog.Info("->", "id", state.ID, "block", i, "offset", state.Offset)

		n, err := in.Read(buffer)
		if err == io.EOF {
			break
		}

		err = stream.Send(&tv1.UploadRequest{
			Id:     state.ID,
			Offset: state.Offset,
			Data:   buffer[:n],
		})
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		state.Offset += int64(n)

		if c.quitAfter > 0 && i == c.quitAfter-1 {
			slog.Info("quitting after QuitAfter blocks", "quitAfter", c.quitAfter)
			slog.Info("kill client within 10 seconds if you want to simulate abrupt halt")
			time.Sleep(10 * time.Second)
			break
		}
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("error closing connection: %w", err)
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
			BlockSize: c.clampBlockSize(resp.PreferredBlocksize),
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
		BlockSize: c.clampBlockSize(resp.PreferredBlocksize),
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

func (c *Client) clampBlockSize(bs int64) int64 {
	return min(max(bs, minBlockSize), maxBlockSize)
}
