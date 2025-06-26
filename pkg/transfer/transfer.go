// Package transfer implements the gRPC service for uploads.
package transfer

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
	"github.com/borud/large-file-upload/pkg/upload"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Service implements the upload service
type Service struct {
	UploadManager      *upload.Manager
	PreferredBlockSize int64
}

// CreateUpload creates a new upload and assigns it an ID.
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

// Upload creates an upload stream.
func (s *Service) Upload(stream tv1.TransferService_UploadServer) error {
	var up *upload.Upload

	var peerAddr string
	peer, ok := peer.FromContext(stream.Context())
	if ok {
		peerAddr = peer.Addr.String()
	}

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// if the stream ends before we have the entire file, that's an error condition.
			if up == nil || up.Offset() != up.Size {
				slog.Error("transfer stopped", "peer", peerAddr)
				return status.Error(codes.FailedPrecondition, "upload incomplete")
			}

			s.UploadManager.Finish(up.ID)
			return stream.SendAndClose(&tv1.UploadResponse{})
		}
		if err != nil {
			return status.Error(codes.Unknown, err.Error())
		}

		// if this is the first message we have to get the upload instance
		if up == nil {
			up = s.UploadManager.GetUpload(req.Id)
			if up == nil {
				return status.Error(codes.NotFound, "upload id not found")
			}
		}

		// ensure the offset is correct
		if up.Offset() != req.Offset {
			return status.Error(codes.FailedPrecondition, fmt.Sprintf("offset mismatch, server=%d, client=%d", up.Offset(), req.Offset))
		}

		// write the data
		n, err := up.Write(req.Data)
		if err != nil {
			return status.Error(codes.Unknown, fmt.Sprintf("write error: %v", err))
		}

		// validate that we wrote the data
		if n != len(req.Data) {
			return status.Error(codes.Unknown, fmt.Sprintf("wrote too few bytes, should write %d but wrote %d", len(req.Data), n))
		}
		slog.Info("wrote block", "id", req.Id, "offset", up.Offset(), "size", n)
	}
}

// GetOffset returns the current offset for an active upload identified by req.Id.
func (s *Service) GetOffset(_ context.Context, req *tv1.GetOffsetRequest) (*tv1.GetOffsetResponse, error) {
	upload := s.UploadManager.GetUpload(req.Id)
	if upload == nil {
		return nil, status.Error(codes.NotFound, "upload not found")
	}

	return &tv1.GetOffsetResponse{Offset: upload.Offset()}, nil
}
