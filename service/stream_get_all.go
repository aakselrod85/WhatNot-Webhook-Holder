package service

type GetChannelStreamsRequest struct {
	ChannelId int64 `json:"channel_id"`
	Page      int64 `json:"page"`
	PageSize  int64 `json:"page_size"`
}

type GetStreamResponse struct {
	Id        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	IsEnded   bool   `json:"is_ended"`
}

type GetChannelStreamsResponse struct {
	Streams []*GetStreamResponse `json:"streams"`
	Total   int64                `json:"total"`
}

func (s *Service) GetChannelStreams(r *GetChannelStreamsRequest) (*GetChannelStreamsResponse, error) {
	pageSize := r.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	page := r.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	streams, err := s.StreamRepositorier.GetByChannelIdPaginated(r.ChannelId, pageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := s.StreamRepositorier.CountByChannelId(r.ChannelId)
	if err != nil {
		return nil, err
	}

	streamResponses := make([]*GetStreamResponse, len(streams))
	for i, stream := range streams {
		streamResponse := GetStreamResponse{}
		streamResponse.Id = stream.Id
		streamResponse.CreatedAt = stream.CreatedAt.UnixMilli()
		streamResponse.Name = stream.Name
		streamResponse.IsEnded = stream.IsEnded
		streamResponses[i] = &streamResponse
	}

	return &GetChannelStreamsResponse{Streams: streamResponses, Total: total}, nil
}
