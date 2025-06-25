package transfer

import (
	"context"
	"fmt"
	"log/slog"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
	"github.com/borud/large-file-upload/pkg/upload"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	UploadManager      *upload.Manager
	PreferredBlockSize int64
}

func (s *Service) CreateUpload(_ context.Context, req *tv1.CreateUploadRequest) (*tv1.CreateUploadResponse, error) {
	upload, err := s.UploadManager.CreateUpload(req.Size, []byte{0})
	if err != nil {
		slog.Error("error creating upload", "err", err)
		return nil, status.Error(codes.NotFound, fmt.Sprintf("error creating upload: %v", err))
	}

	return &tv1.CreateUploadResponse{
		Id:                 upload.ID,
		PreferredBlocksize: s.PreferredBlockSize,
	}, nil
}

func (s *Service) UploadBlock(_ context.Context, req *tv1.UploadBlockRequest) (*tv1.UploadBlockResponse, error) {
	upload := s.UploadManager.GetUpload(req.Id)
	if upload == nil {
		slog.Error("unknown upload", "id", req.Id)
		return nil, status.Error(codes.NotFound, fmt.Sprintf("ID [%s] not found", req.Id))
	}

	_, err := upload.Write(req.Data)
	if err != nil {
		slog.Error("error writing block", "id", upload.ID, "filename", upload.Filename, "err", err)

		err2 := s.UploadManager.Finish(upload.ID)
		if err2 != nil {
			slog.Error("error writing finishing upload", "id", upload.ID, "filename", upload.Filename, "err", err)
		}

		return nil, status.Error(codes.Internal, fmt.Sprintf("error writing block: %v", err))
	}

	return &tv1.UploadBlockResponse{}, nil
}
