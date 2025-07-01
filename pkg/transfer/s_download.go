package transfer

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Download file by id starting at offset.
func (s *Service) Download(req *tv1.DownloadRequest, stream tv1.TransferService_DownloadServer) error {
	req.PreferredBlocksize = clampBlockSize(req.PreferredBlocksize)

	slog.Info("download", "id", req.Id, "offset", req.Offset, "blocksize", req.PreferredBlocksize)

	id, err := ParseID(req.Id)
	if err != nil {
		slog.Error("error parsing id", "id", req.Id, "err", err)
		status.Error(codes.InvalidArgument, fmt.Sprintf("invalid id: %v", err))
	}

	in, err := s.fileStore.OpenReadOnly(id)
	if err != nil {
		slog.Error("error opening file", "id", id, "err", err)
		status.Error(codes.Internal, fmt.Sprintf("error opening file for id [%s]: %v", id, err))
	}
	defer in.Close()

	buffer := make([]byte, req.PreferredBlocksize)

	for {
		n, err := in.Read(buffer)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			slog.Error("error reading file", "id", id, "err", err)
			return status.Error(codes.Internal, fmt.Sprintf("error reading file for id [%s]: %v", id, err))
		}

		checksum := sha256.Sum256(buffer[:n])

		err = stream.Send(&tv1.DownloadResponse{Sha256: checksum[:], Data: buffer[:n]})
		if err != nil {
			slog.Error("error sending block", "id", id, "path", in.Name(), "err", err)
			return status.Error(codes.Internal, fmt.Sprintf("error sending block for id [%s]: %v", id, err))
		}
	}

	return nil
}
