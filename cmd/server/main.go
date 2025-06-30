// Package main is the server
package main

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"

	"github.com/alecthomas/kong"
	"github.com/borud/large-file-upload/pkg/transfer"
	"google.golang.org/grpc"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
)

var opt struct {
	ListenAddr string `kong:"help='GRPC listen addr',default=':4200',required"`
	Incoming   string `kong:"help='incoming dir',default='incoming',required"`
	Blocksize  int64  `kong:"help='set preferred block size',default='1048576'"`
}

func main() {
	kong.Parse(&opt)

	transferService, err := transfer.NewService(transfer.Config{
		IncomingDir:        opt.Incoming,
		PreferredBlockSize: opt.Blocksize,
		UploadFinishedHook: uploadFinished,
		UploadProgressHook: uploadProgress,
		UploadCreatedHook:  uploadCreated,
	})
	if err != nil {
		slog.Error("error creating transfer service", "err", err)
		return
	}

	grpcServer := grpc.NewServer()

	tv1.RegisterTransferServiceServer(grpcServer, transferService)
	grpcListener, err := net.Listen("tcp", opt.ListenAddr)
	if err != nil {
		slog.Error("error creating listening socket", "listenAddr", opt.ListenAddr, "err", err)
		return
	}

	slog.Info("starting gRPC server", "listenAddr", opt.ListenAddr)
	err = grpcServer.Serve(grpcListener)
	if err != nil {
		slog.Error("error exiting gRPC server", "listenAddr", opt.ListenAddr, "err", err)
	}
}

func uploadCreated(filename string, size int64, offset int64, metadata []byte) {
	slog.Info("upload created", "filename", filename, "size", size, "offset", offset, "metadata", hex.EncodeToString(metadata))
}

func uploadFinished(filename string, size int64, offset int64, _ []byte) {
	slog.Info("upload finished", "filename", filename, "size", size, "offset", offset)
}

func uploadProgress(filename string, size int64, offset int64, _ []byte) {
	percent := fmt.Sprintf("%.1f%%", float64(offset*100)/float64(size))
	slog.Info("progress", "filename", filename, "percent", percent)
}
