// Package main is the server
package main

import (
	"log/slog"
	"net"
	"time"

	"github.com/alecthomas/kong"
	"github.com/borud/large-file-upload/pkg/transfer"
	"github.com/borud/large-file-upload/pkg/upload"
	"google.golang.org/grpc"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
)

var opt struct {
	ListenAddr string `kong:"help='GRPC listen addr',default=':4200',required"`
	Incoming   string `kong:"help='incoming dir',default='incoming',required"`
	Archive    string `kong:"help='archive dir',default='archive',required"`
}

func main() {
	kong.Parse(&opt)

	uploadManager, err := upload.NewManager(opt.Incoming, opt.Archive)
	if err != nil {
		slog.Error("error creating upload manager", "err", err)
	}

	transferService := &transfer.Service{
		UploadManager:      uploadManager,
		PreferredBlockSize: 1024 * 1024,
	}

	grpcServer := grpc.NewServer()

	tv1.RegisterTransferServiceServer(grpcServer, transferService)
	grpcListener, err := net.Listen("tcp", opt.ListenAddr)
	if err != nil {
		slog.Error("error creating listening socket", "listenAddr", opt.ListenAddr, "err", err)
		return
	}

	slog.Info("starting gRPC server", "listenAddr", opt.ListenAddr)
	err = grpcServer.Serve(&timeoutListener{
		Listener: grpcListener,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		slog.Error("error exiting gRPC server", "listenAddr", opt.ListenAddr, "err", err)
	}
}
