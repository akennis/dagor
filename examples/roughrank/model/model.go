package model

import "fmt"

type Request struct {
	UserId    int64
	StreamIds []int64
}

type Stream struct {
	StreamId int64
	Score    float64
}

func (r *Stream) String() string {
	return fmt.Sprintf("Stream{StreamId: %d, Score: %f}", r.StreamId, r.Score)
}

type Response struct {
	UserId  int64
	Streams []*Stream
}

func (r *Response) String() string {
	return fmt.Sprintf("Response{UserId: %d, Streams: %v}", r.UserId, r.Streams)
}

type Score struct {
	StreamId int64
	Score    float64
}

type StreamEmb struct {
	Id  int64
	Emb []float64
}
