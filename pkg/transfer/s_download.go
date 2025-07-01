package transfer

import (
	"log/slog"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
)

// Download file by id starting at offset.
func (s *Service) Download(req *tv1.DownloadRequest, stream tv1.TransferService_DownloadServer) error {
	slog.Info("download", "id", req.Id, "offset", req.Offset, "blocksize", req.PreferredBlocksize)

	stream.Send(&tv1.DownloadResponse{
		Sha256: []byte{},
		Data:   []byte{},
	})
	return nil
}
