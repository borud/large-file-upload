package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"

	tv1 "github.com/borud/large-file-upload/gen/transfer/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// CreateUpload creates a new upload and assigns it an ID.
func (s *Service) CreateUpload(_ context.Context, req *tv1.CreateUploadRequest) (*tv1.CreateUploadResponse, error) {
	upload, err := s.UploadManager.CreateUpload(req.Size, []byte{0})
	if err != nil {
		slog.Error("error creating upload", "err", err)
		return nil, status.Error(codes.NotFound, fmt.Sprintf("error creating upload: %v", err))
	}

	if s.config.UploadCreatedHook != nil {
		s.config.UploadCreatedHook(upload.Filename, upload.Size, upload.Offset(), upload.Metadata)
	}

	return &tv1.CreateUploadResponse{
		Id:                 upload.ID,
		PreferredBlocksize: s.config.PreferredBlockSize,
	}, nil
}

// Upload creates an upload stream.
func (s *Service) Upload(stream tv1.TransferService_UploadServer) error {
	var up *upload

	var peerAddr string
	peer, ok := peer.FromContext(stream.Context())
	if ok {
		peerAddr = peer.Addr.String()
	}

	for {
		req, err := stream.Recv()

		if err == io.EOF {
			// if the stream ends before we have the entire file, that's an error condition.
			if up == nil {
				slog.Error("transfer stopped on first block from", "peer", peerAddr)
				return status.Error(codes.FailedPrecondition, "upload failed on first block")
			}

			// we did not get whole file
			if up.Offset() != up.Size {
				slog.Error("transfer stopped (EOF)", "id", up.ID, "peer", peerAddr)
				return status.Error(codes.FailedPrecondition, "upload incomplete")
			}

			// Invariant: if we are here the upload succeeded

			if s.config.UploadFinishedHook != nil {
				s.config.UploadFinishedHook(up.Filename, up.Size, up.Offset(), up.Metadata)
			}

			err := s.UploadManager.Finish(up.ID)
			if err != nil {
				// not much we can do about this except report it
				slog.Error("error finishing upload", "err", err)
			}

			return stream.SendAndClose(&tv1.UploadResponse{})
		}

		if err != nil {
			slog.Error("transfer stopped", "peer", peerAddr, "err", err)
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

		// ensure checksum is correct
		verifyChecksum := sha256.Sum256(req.Data)
		if !bytes.Equal(verifyChecksum[:], req.Sha256) {
			return status.Error(codes.DataLoss, "checksums did not match")
		}

		// write the data to the file
		n, err := up.Write(req.Data)
		if err != nil {
			return status.Error(codes.Unknown, fmt.Sprintf("write error: %v", err))
		}

		// validate that we wrote the data
		if n != len(req.Data) {
			return status.Error(codes.Unknown, fmt.Sprintf("wrote too few bytes, should write %d but wrote %d", len(req.Data), n))
		}

		if s.config.UploadProgressHook != nil {
			s.config.UploadProgressHook(up.Filename, up.Size, up.Offset(), up.Metadata)
		}

		slog.Debug("wrote block", "id", req.Id, "offset", up.Offset(), "size", n, "checksum", hex.EncodeToString(verifyChecksum[:]))
	}
}

// GetOffset returns the current offset for an active upload identified by req.Id.
func (s *Service) GetOffset(_ context.Context, req *tv1.GetOffsetRequest) (*tv1.GetOffsetResponse, error) {
	upload := s.UploadManager.GetUpload(req.Id)
	if upload == nil {
		return nil, status.Error(codes.NotFound, "upload not found")
	}

	return &tv1.GetOffsetResponse{Offset: upload.Offset(), PreferredBlocksize: s.config.PreferredBlockSize}, nil
}
