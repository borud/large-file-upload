// Package main is the client.
package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
)

var opt struct {
	ServerAddr string `kong:"help='GRPC server addr',default=':4200',required"`
	Filename   string `kong:"arg,help='file to be uploaded',required"`
	QuitAfter  int    `kong:"help='quit after N blocks'"`
}

const (
	minBlockSize = 512 * 1024
	maxBlockSize = 1024 * 1024
)

func main() {
	kong.Parse(&opt)

	conn, err := grpc.NewClient(
		opt.ServerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		slog.Error("error connecting", "serverAddr", opt.ServerAddr, "err", err)
		return
	}
	defer conn.Close()

	client := tv1.NewTransferServiceClient(conn)

	fileInfo, err := os.Stat(opt.Filename)
	if err != nil {
		slog.Error("error getting file size", "filename", opt.Filename, "err", err)
		return
	}

	in, err := os.Open(opt.Filename)
	if err != nil {
		slog.Error("error opening file", "filename", opt.Filename, "err", err)
		return
	}
	defer in.Close()

	// Create the upload
	resp, err := client.CreateUpload(context.Background(), &tv1.CreateUploadRequest{
		Size:     fileInfo.Size(),
		Metadata: []byte{1, 2, 3},
	})
	if err != nil {
		slog.Error("error creating upload", "err", err)
		return
	}

	// set block size
	blockSize := min(max(resp.PreferredBlocksize, minBlockSize), maxBlockSize)

	stream, err := client.Upload(context.Background())
	if err != nil {
		slog.Error("unable to create upload stream", "err", err)
		return
	}

	buffer := make([]byte, blockSize)
	offset := int64(0)

	for i := 0; ; i++ {
		slog.Info("block", "offset", offset)
		n, err := in.Read(buffer)
		if err == io.EOF {
			break
		}

		err = stream.Send(&tv1.UploadRequest{
			Id:     resp.Id,
			Offset: offset,
			Data:   buffer[:n],
		})
		if err != nil {
			slog.Error("stream.Send failed", "offset", offset, "err", err)
			return
		}

		offset += int64(n)

		if opt.QuitAfter > 0 && i == opt.QuitAfter-1 {
			slog.Info("quitting after QuitAfter blocks", "quitAfter", opt.QuitAfter)
			slog.Info("kill client within 10 seconds if you want to simulate abrupt halt")
			time.Sleep(10 * time.Second)
			break
		}
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		slog.Error("error closing stream", "err", err)
	}
}
