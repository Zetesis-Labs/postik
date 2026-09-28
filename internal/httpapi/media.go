package httpapi

import (
	"context"
	"errors"
	"io"

	core "github.com/zetesis-labs/postik/internal/core/media"
	"github.com/zetesis-labs/postik/internal/library"
	"github.com/zetesis-labs/postik/internal/postgres"
)

var (
	errMediaNotFound    = errorBody("not_found", "Media not found")
	errUnsupportedMedia = errorBody("unsupported_type", "This file type is not supported")
	errMediaTooLarge    = errorBody("too_large", "The file is too large")
	errMediaUnreadable  = errorBody("unreadable", "The file could not be read")
)

func toMedia(m postgres.Media) Media {
	out := Media{
		Id:               m.ID,
		Name:             m.Name,
		Url:              m.Path,
		Kind:             MediaKind(m.Kind),
		Mime:             m.Mime,
		Size:             m.Size,
		Alt:              m.Alt,
		ThumbnailUrl:     m.ThumbnailPath,
		ThumbnailSeconds: m.ThumbnailSeconds,
		CreatedAt:        m.CreatedAt,
	}
	return out
}

func (s *Server) ListMedia(ctx context.Context, request ListMediaRequestObject) (ListMediaResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return ListMedia401JSONResponse{errUnauthenticated}, err
	}
	page := 1
	if request.Params.Page != nil && *request.Params.Page > 1 {
		page = *request.Params.Page
	}
	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}
	items, total, err := s.Media.Store.List(ctx, org, search, page, core.PageSize)
	if err != nil {
		return nil, err
	}
	out := ListMedia200JSONResponse{Items: make([]Media, len(items)), Page: page, Pages: core.Pages(total), Total: total}
	for i, m := range items {
		out.Items[i] = toMedia(m)
	}
	return out, nil
}

func (s *Server) UploadMedia(ctx context.Context, request UploadMediaRequestObject) (UploadMediaResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return UploadMedia401JSONResponse(errUnauthenticated), err
	}
	for {
		part, err := request.Body.NextPart()
		if errors.Is(err, io.EOF) {
			return UploadMedia400JSONResponse{errMediaUnreadable}, nil
		}
		if err != nil {
			return UploadMedia400JSONResponse{errMediaUnreadable}, nil
		}
		if part.FormName() != "file" {
			continue
		}
		media, err := s.Media.Upload(ctx, org, part.FileName(), part)
		switch {
		case errors.Is(err, library.ErrUnsupported):
			return UploadMedia415JSONResponse(errUnsupportedMedia), nil
		case errors.Is(err, library.ErrTooLarge):
			return UploadMedia413JSONResponse(errMediaTooLarge), nil
		case errors.Is(err, library.ErrUnreadable):
			return UploadMedia400JSONResponse{errMediaUnreadable}, nil
		case err != nil:
			return nil, err
		}
		return UploadMedia201JSONResponse(toMedia(*media)), nil
	}
}

func (s *Server) UpdateMedia(ctx context.Context, request UpdateMediaRequestObject) (UpdateMediaResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return UpdateMedia401JSONResponse{errUnauthenticated}, err
	}
	media, err := s.Media.Store.Get(ctx, org, request.Id)
	if errors.Is(err, postgres.ErrNotFound) {
		return UpdateMedia404JSONResponse(errMediaNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	body := request.Body
	if body.Alt != nil {
		media.Alt = *body.Alt
	}
	if body.ThumbnailMediaId != nil {
		thumbnail, err := s.Media.Store.Get(ctx, org, *body.ThumbnailMediaId)
		if errors.Is(err, postgres.ErrNotFound) || (err == nil && thumbnail.Kind != string(core.Image)) {
			return UpdateMedia404JSONResponse(errMediaNotFound), nil
		}
		if err != nil {
			return nil, err
		}
		media.ThumbnailPath = &thumbnail.Path
	}
	if body.ThumbnailSeconds != nil {
		media.ThumbnailSeconds = body.ThumbnailSeconds
	}
	if err := s.Media.Store.Update(ctx, media); err != nil {
		return nil, err
	}
	return UpdateMedia200JSONResponse(toMedia(*media)), nil
}

func (s *Server) DeleteMedia(ctx context.Context, request DeleteMediaRequestObject) (DeleteMediaResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return DeleteMedia401JSONResponse{errUnauthenticated}, err
	}
	err = s.Media.Store.SoftDelete(ctx, org, request.Id, s.Now())
	if errors.Is(err, postgres.ErrNotFound) {
		return DeleteMedia404JSONResponse(errMediaNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return DeleteMedia204Response{}, nil
}
