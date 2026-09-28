package httpapi

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/auth"
	"github.com/zetesis-labs/postik/internal/core/posts"
	"github.com/zetesis-labs/postik/internal/postgres"
)

const (
	postPageSize   = 100
	excerptLength  = 120
	codeInvalid    = "invalid_post"
	codeMedia      = "media_unavailable"
	codeTag        = "tag_unavailable"
	messageInvalid = "The post does not pass validation"
)

var (
	errPostNotFound      = errorBody("not_found", "Post not found")
	errTagNotFound       = errorBody("not_found", "Tag not found")
	errTagTaken          = errorBody("tag_taken", "A tag with that name already exists")
	errTagInvalid        = errorBody("invalid_tag", "A tag needs a name and a color")
	errRepublishRequired = errorBody("republish_required", "The post was already published; confirm to publish it again")
	errPastDate          = errorBody(posts.CodePastDate, "The date has already passed")
	errNoSlot            = errorBody("no_slot", "There is no free slot")
	errReleaseNotMissing = errorBody("release_not_missing", "The post is not published or already has a link")
	errReleaseURL        = errorBody("invalid_url", "The link must be an https URL")
)

// ---- Tags ----

func toTag(t postgres.Tag) Tag {
	return Tag{Id: t.ID, Name: t.Name, Color: t.Color}
}

func toTags(list []postgres.Tag) []Tag {
	out := make([]Tag, len(list))
	for i, t := range list {
		out[i] = toTag(t)
	}
	return out
}

func validTag(input TagInput) (string, string, bool) {
	name, color := strings.TrimSpace(input.Name), strings.TrimSpace(input.Color)
	return name, color, name != "" && color != ""
}

func (s *Server) ListTags(ctx context.Context, _ ListTagsRequestObject) (ListTagsResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return ListTags401JSONResponse{errUnauthenticated}, err
	}
	tags, err := s.Posts.Tags(ctx, org)
	if err != nil {
		return nil, err
	}
	return ListTags200JSONResponse(toTags(tags)), nil
}

func (s *Server) CreateTag(ctx context.Context, request CreateTagRequestObject) (CreateTagResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return CreateTag401JSONResponse(errUnauthenticated), err
	}
	name, color, valid := validTag(*request.Body)
	if !valid {
		return CreateTag400JSONResponse{errTagInvalid}, nil
	}
	tag := postgres.Tag{ID: uuid.New(), OrganizationID: org, Name: name, Color: color, CreatedAt: s.Now()}
	err = s.Posts.CreateTag(ctx, &tag)
	if errors.Is(err, postgres.ErrConflict) {
		return CreateTag409JSONResponse(errTagTaken), nil
	}
	if err != nil {
		return nil, err
	}
	return CreateTag201JSONResponse(toTag(tag)), nil
}

func (s *Server) UpdateTag(ctx context.Context, request UpdateTagRequestObject) (UpdateTagResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return UpdateTag401JSONResponse(errUnauthenticated), err
	}
	name, color, valid := validTag(*request.Body)
	if !valid {
		return UpdateTag400JSONResponse{errTagInvalid}, nil
	}
	tag := postgres.Tag{ID: request.Id, OrganizationID: org, Name: name, Color: color}
	switch err := s.Posts.UpdateTag(ctx, &tag); {
	case errors.Is(err, postgres.ErrNotFound):
		return UpdateTag404JSONResponse(errTagNotFound), nil
	case errors.Is(err, postgres.ErrConflict):
		return UpdateTag409JSONResponse(errTagTaken), nil
	case err != nil:
		return nil, err
	}
	return UpdateTag200JSONResponse(toTag(tag)), nil
}

func (s *Server) DeleteTag(ctx context.Context, request DeleteTagRequestObject) (DeleteTagResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return DeleteTag401JSONResponse{errUnauthenticated}, err
	}
	err = s.Posts.DeleteTag(ctx, org, request.Id)
	if errors.Is(err, postgres.ErrNotFound) {
		return DeleteTag404JSONResponse(errTagNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return DeleteTag204Response{}, nil
}

// ---- Resolution of what a submission points to ----

// resolved holds the organization's view of what a submission references,
// plus the problems found while resolving it.
type resolved struct {
	channels map[uuid.UUID]*postgres.Channel
	media    map[uuid.UUID]postgres.PostMedia
	tags     []uuid.UUID
	problems []posts.Problem
}

func (s *Server) resolve(ctx context.Context, org uuid.UUID, channelIDs []uuid.UUID, values [][]PostValueInput, tagIDs []uuid.UUID) (resolved, error) {
	r := resolved{channels: map[uuid.UUID]*postgres.Channel{}, media: map[uuid.UUID]postgres.PostMedia{}}
	for _, id := range channelIDs {
		channel, err := s.Channels.Get(ctx, org, id)
		if errors.Is(err, postgres.ErrNotFound) {
			continue
		}
		if err != nil {
			return r, err
		}
		r.channels[id] = channel
	}
	for _, list := range values {
		for _, v := range list {
			for _, ref := range v.Media {
				if _, done := r.media[ref.Id]; done {
					continue
				}
				m, err := s.Media.Store.Get(ctx, org, ref.Id)
				if errors.Is(err, postgres.ErrNotFound) {
					r.problems = append(r.problems, posts.Problem{Code: codeMedia})
					continue
				}
				if err != nil {
					return r, err
				}
				r.media[ref.Id] = postgres.PostMedia{ID: m.ID, URL: m.Path, Kind: m.Kind, Alt: m.Alt, ThumbnailURL: m.ThumbnailPath, ThumbnailSeconds: m.ThumbnailSeconds}
			}
		}
	}
	tags, err := s.Posts.TagsByID(ctx, org, tagIDs)
	if err != nil {
		return r, err
	}
	if len(tags) != len(uniqueIDs(tagIDs)) {
		r.problems = append(r.problems, posts.Problem{Code: codeTag})
	}
	for _, t := range tags {
		r.tags = append(r.tags, t.ID)
	}
	return r, nil
}

func uniqueIDs(ids []uuid.UUID) map[uuid.UUID]bool {
	set := map[uuid.UUID]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func (r resolved) channelSubmission(id uuid.UUID, values []PostValueInput) posts.ChannelSubmission {
	sub := posts.ChannelSubmission{ChannelID: id}
	if channel, ok := r.channels[id]; ok {
		sub.Provider = channel.Provider
		sub.Available = !channel.Disabled && !channel.InBetweenSteps
	}
	for _, v := range values {
		value := posts.Value{Content: v.Content}
		for _, ref := range v.Media {
			if m, ok := r.media[ref.Id]; ok {
				value.Media = append(value.Media, m.Kind)
			}
		}
		sub.Values = append(sub.Values, value)
	}
	return sub
}

func (r resolved) storedValues(values []PostValueInput) []postgres.PostValue {
	out := make([]postgres.PostValue, len(values))
	for i, v := range values {
		media := make([]postgres.PostMedia, 0, len(v.Media))
		for _, ref := range v.Media {
			media = append(media, r.media[ref.Id])
		}
		out[i] = postgres.PostValue{Content: v.Content, DelayMinutes: max(v.DelayMinutes, 0), Media: media}
	}
	return out
}

func invalid(problems []posts.Problem) InvalidPostJSONResponse {
	out := InvalidPostJSONResponse{Code: codeInvalid, Message: messageInvalid, Problems: make([]Problem, len(problems))}
	for i, p := range problems {
		out.Problems[i] = Problem{Code: p.Code, ChannelId: p.ChannelID, ValueIndex: p.ValueIndex}
	}
	return out
}

// ---- Posts ----

func (s *Server) CreatePosts(ctx context.Context, request CreatePostsRequestObject) (CreatePostsResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return CreatePosts401JSONResponse{errUnauthenticated}, err
	}
	body := request.Body
	kind := posts.Type(body.Type)
	channelIDs := make([]uuid.UUID, len(body.Posts))
	values := make([][]PostValueInput, len(body.Posts))
	for i, p := range body.Posts {
		channelIDs[i], values[i] = p.ChannelId, p.Values
	}
	r, err := s.resolve(ctx, org, channelIDs, values, body.Tags)
	if err != nil {
		return nil, err
	}
	now := s.Now()
	publishAt := posts.PublishAt(kind, body.PublishAt, now)
	submission := posts.Submission{Draft: kind == posts.TypeDraft, CheckDate: kind == posts.TypeSchedule, PublishAt: publishAt}
	for i, p := range body.Posts {
		submission.Channels = append(submission.Channels, r.channelSubmission(p.ChannelId, values[i]))
	}
	problems := append(r.problems, posts.Validate(submission, now)...)
	if len(problems) > 0 {
		return CreatePosts400JSONResponse{invalid(problems)}, nil
	}

	status := posts.Scheduled
	if kind == posts.TypeDraft {
		status = posts.Draft
	}
	group := postgres.PostGroup{ID: uuid.New(), OrganizationID: org, Origin: "web", CreatedBy: currentUser(ctx), CreatedAt: now}
	created := make([]postgres.Post, len(body.Posts))
	out := CreatePosts201JSONResponse{GroupId: group.ID}
	for i, p := range body.Posts {
		created[i] = postgres.Post{
			ID: uuid.New(), OrganizationID: org, GroupID: group.ID, ChannelID: p.ChannelId,
			Status: string(status), PublishAt: publishAt, Values: r.storedValues(p.Values), Settings: settingsOrEmpty(p.Settings),
			CreatedAt: now, UpdatedAt: now,
		}
		out.Posts = append(out.Posts, struct {
			ChannelId uuid.UUID `json:"channelId"`
			Id        uuid.UUID `json:"id"`
		}{ChannelId: p.ChannelId, Id: created[i].ID})
	}
	if err := s.Posts.CreateGroup(ctx, group, r.tags, created); err != nil {
		return nil, err
	}
	return out, nil
}

func currentUser(ctx context.Context) *uuid.UUID {
	if p, ok := auth.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return nil
}

func settingsOrEmpty(settings map[string]interface{}) map[string]any {
	if settings == nil {
		return map[string]any{}
	}
	return settings
}

func toPostChannel(c *postgres.Channel) PostChannel {
	if c == nil {
		return PostChannel{}
	}
	return PostChannel{Id: c.ID, Name: c.Name, Provider: c.Provider, Picture: c.Picture, CustomerId: c.CustomerID}
}

func (s *Server) calendarPosts(ctx context.Context, list []postgres.Post) ([]CalendarPost, error) {
	groupIDs := make([]uuid.UUID, 0, len(list))
	for _, p := range list {
		groupIDs = append(groupIDs, p.GroupID)
	}
	tags, err := s.Posts.GroupTags(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	out := make([]CalendarPost, len(list))
	for i, p := range list {
		excerpt := ""
		if len(p.Values) > 0 {
			excerpt = posts.Excerpt(p.Values[0].Content, excerptLength)
		}
		out[i] = CalendarPost{
			Id: p.ID, GroupId: p.GroupID, Status: CalendarPostStatus(p.Status), PublishAt: p.PublishAt.UTC(),
			Excerpt: excerpt, Channel: toPostChannel(p.Channel), Tags: toTags(tags[p.GroupID]),
			ReleaseUrl: p.ReleaseURL, Error: p.Error,
		}
	}
	return out, nil
}

func (s *Server) ListCalendarPosts(ctx context.Context, request ListCalendarPostsRequestObject) (ListCalendarPostsResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return ListCalendarPosts401JSONResponse{errUnauthenticated}, err
	}
	list, err := s.Posts.Range(ctx, org, request.Params.From, request.Params.To, request.Params.Customer)
	if err != nil {
		return nil, err
	}
	out, err := s.calendarPosts(ctx, list)
	if err != nil {
		return nil, err
	}
	return ListCalendarPosts200JSONResponse(out), nil
}

func (s *Server) ListPosts(ctx context.Context, request ListPostsRequestObject) (ListPostsResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return ListPosts401JSONResponse{errUnauthenticated}, err
	}
	page := 1
	if request.Params.Page != nil && *request.Params.Page > 1 {
		page = *request.Params.Page
	}
	status := ""
	if request.Params.Status != nil && *request.Params.Status != ListPostsParamsStatusAll {
		status = string(*request.Params.Status)
	}
	list, total, err := s.Posts.Page(ctx, org, status, page, postPageSize)
	if err != nil {
		return nil, err
	}
	items, err := s.calendarPosts(ctx, list)
	if err != nil {
		return nil, err
	}
	pages := (total + postPageSize - 1) / postPageSize
	return ListPosts200JSONResponse{Items: items, Page: page, Pages: max(pages, 1), Total: total}, nil
}

func (s *Server) NextSlot(ctx context.Context, request NextSlotRequestObject) (NextSlotResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return NextSlot401JSONResponse{errUnauthenticated}, err
	}
	var times []int
	if request.Params.ChannelId != nil {
		channel, err := s.Channels.Get(ctx, org, *request.Params.ChannelId)
		if errors.Is(err, postgres.ErrNotFound) {
			return NextSlot404JSONResponse(errChannelNotFound), nil
		}
		if err != nil {
			return nil, err
		}
		times = channel.PostingTimes
	} else {
		list, err := s.Channels.List(ctx, org)
		if err != nil {
			return nil, err
		}
		for _, c := range list {
			if !c.Disabled {
				times = append(times, c.PostingTimes...)
			}
		}
	}
	now := s.Now()
	start := now.UTC().Truncate(24 * time.Hour)
	taken, err := s.Posts.Taken(ctx, org, start, start.AddDate(0, 0, posts.SlotSearchDays+1))
	if err != nil {
		return nil, err
	}
	slot, found := posts.NextFreeSlot(times, taken, now)
	if !found {
		return NextSlot404JSONResponse(errNoSlot), nil
	}
	return NextSlot200JSONResponse{Date: slot}, nil
}

func (s *Server) postDetail(ctx context.Context, p *postgres.Post) (PostDetail, error) {
	tags, err := s.Posts.GroupTags(ctx, []uuid.UUID{p.GroupID})
	if err != nil {
		return PostDetail{}, err
	}
	values := make([]PostValue, len(p.Values))
	for i, v := range p.Values {
		media := make([]PostMedia, len(v.Media))
		for j, m := range v.Media {
			media[j] = PostMedia{Id: m.ID, Url: m.URL, Kind: m.Kind, Alt: &m.Alt, ThumbnailUrl: m.ThumbnailURL, ThumbnailSeconds: m.ThumbnailSeconds}
		}
		values[i] = PostValue{Content: v.Content, DelayMinutes: v.DelayMinutes, Media: media}
	}
	return PostDetail{
		Id: p.ID, GroupId: p.GroupID, Status: PostDetailStatus(p.Status), PublishAt: p.PublishAt.UTC(),
		Channel: toPostChannel(p.Channel), Values: values, Settings: settingsOrEmpty(p.Settings),
		Tags: toTags(tags[p.GroupID]), ReleaseUrl: p.ReleaseURL, Error: p.Error,
	}, nil
}

func (s *Server) GetPost(ctx context.Context, request GetPostRequestObject) (GetPostResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return GetPost401JSONResponse{errUnauthenticated}, err
	}
	post, err := s.Posts.Get(ctx, org, request.Id)
	if errors.Is(err, postgres.ErrNotFound) {
		return GetPost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	detail, err := s.postDetail(ctx, post)
	if err != nil {
		return nil, err
	}
	return GetPost200JSONResponse(detail), nil
}

func (s *Server) UpdatePost(ctx context.Context, request UpdatePostRequestObject) (UpdatePostResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return UpdatePost401JSONResponse{errUnauthenticated}, err
	}
	post, err := s.Posts.Get(ctx, org, request.Id)
	if errors.Is(err, postgres.ErrNotFound) {
		return UpdatePost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	body := request.Body
	mode := posts.Mode(body.Mode)
	republish := body.Republish != nil && *body.Republish
	status, err := posts.AfterEdit(posts.Status(post.Status), mode, republish)
	if errors.Is(err, posts.ErrRepublishRequired) {
		return UpdatePost409JSONResponse(errRepublishRequired), nil
	}
	r, err := s.resolve(ctx, org, []uuid.UUID{post.ChannelID}, [][]PostValueInput{body.Values}, body.Tags)
	if err != nil {
		return nil, err
	}
	now := s.Now()
	publishAt := posts.PublishAt(posts.TypeSchedule, body.PublishAt, now)
	submission := posts.Submission{
		Draft:     status == posts.Draft,
		CheckDate: mode == posts.ModeSchedule,
		PublishAt: publishAt,
		Channels:  []posts.ChannelSubmission{r.channelSubmission(post.ChannelID, body.Values)},
	}
	if problems := append(r.problems, posts.Validate(submission, now)...); len(problems) > 0 {
		return UpdatePost400JSONResponse{invalid(problems)}, nil
	}
	post.Status = string(status)
	post.PublishAt = publishAt
	post.Values = r.storedValues(body.Values)
	post.Settings = settingsOrEmpty(body.Settings)
	post.UpdatedAt = now
	if err := s.Posts.Save(ctx, post, r.tags); err != nil {
		return nil, err
	}
	detail, err := s.postDetail(ctx, post)
	if err != nil {
		return nil, err
	}
	return UpdatePost200JSONResponse(detail), nil
}

func (s *Server) MovePost(ctx context.Context, request MovePostRequestObject) (MovePostResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return MovePost401JSONResponse(errUnauthenticated), err
	}
	post, err := s.Posts.Get(ctx, org, request.Id)
	if errors.Is(err, postgres.ErrNotFound) {
		return MovePost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	now := s.Now()
	publishAt := posts.PublishAt(posts.TypeSchedule, request.Body.PublishAt, now)
	if posts.IsPast(publishAt, now) {
		return MovePost400JSONResponse{errPastDate}, nil
	}
	republish := request.Body.Republish != nil && *request.Body.Republish
	status, err := posts.AfterMove(posts.Status(post.Status), posts.Mode(request.Body.Mode), republish)
	if errors.Is(err, posts.ErrRepublishRequired) {
		return MovePost409JSONResponse(errRepublishRequired), nil
	}
	post.Status, post.PublishAt, post.UpdatedAt = string(status), publishAt, now
	if err := s.Posts.Move(ctx, post); err != nil {
		return nil, err
	}
	return MovePost204Response{}, nil
}

func (s *Server) DeletePostGroup(ctx context.Context, request DeletePostGroupRequestObject) (DeletePostGroupResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return DeletePostGroup401JSONResponse{errUnauthenticated}, err
	}
	post, err := s.Posts.Get(ctx, org, request.Id)
	if errors.Is(err, postgres.ErrNotFound) {
		return DeletePostGroup404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.Posts.DeleteGroup(ctx, org, post.GroupID); err != nil {
		return nil, err
	}
	return DeletePostGroup204Response{}, nil
}

func (s *Server) LinkPostRelease(ctx context.Context, request LinkPostReleaseRequestObject) (LinkPostReleaseResponseObject, error) {
	org, ok, err := s.activeOrganization(ctx)
	if err != nil || !ok {
		return LinkPostRelease401JSONResponse(errUnauthenticated), err
	}
	link, err := url.Parse(strings.TrimSpace(request.Body.Url))
	if err != nil || link.Scheme != "https" || link.Host == "" {
		return LinkPostRelease400JSONResponse{errReleaseURL}, nil
	}
	switch err := s.Posts.SetReleaseURL(ctx, org, request.Id, link.String(), s.Now()); {
	case errors.Is(err, postgres.ErrNotFound):
		return LinkPostRelease404JSONResponse(errPostNotFound), nil
	case errors.Is(err, postgres.ErrConflict):
		return LinkPostRelease409JSONResponse(errReleaseNotMissing), nil
	case err != nil:
		return nil, err
	}
	return LinkPostRelease204Response{}, nil
}
