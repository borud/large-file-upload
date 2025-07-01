// package main is the client
package main

import (
	"log/slog"

	"github.com/alecthomas/kong"
	"github.com/borud/large-file-upload/pkg/transfer"
)

var opt struct {
	ServerAddr string   `kong:"help='gRPC address of server',default=':4200'"`
	QuitAfter  int      `kong:"help='prematurely quit upload',default='0'"`
	Filenames  []string `kong:"arg,help='files to be uploaded',required"`
}

func main() {
	kong.Parse(&opt)

	client, err := transfer.CreateClient(transfer.ClientConfig{
		ServerAddr: opt.ServerAddr,
		QuitAfter:  opt.QuitAfter,
	})
	if err != nil {
		slog.Error("error creating client", "err", err)
		return
	}

	id := ""

	for _, filename := range opt.Filenames {
		id, err = client.Upload(filename)
		if err != nil {
			slog.Error("error uploading file", "filename", filename, "err", err)
			return
		}
		slog.Info("uploaded file", "filename", filename, "id", id)
	}

	err = client.Download(transfer.ID(id), "outputfile")
	if err != nil {
		slog.Error("error downloading", "err", err)
	}
}
