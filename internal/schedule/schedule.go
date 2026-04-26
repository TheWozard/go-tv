package schedule

import (
	"encoding/json"
	"fmt"
	"os"
)

type Video struct {
	ID string `json:"id"`
}

type Schedule struct {
	Videos []Video `json:"videos"`
}

func Load(path string) (*Schedule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var s Schedule
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Schedule) First() *Video {
	if len(s.Videos) == 0 {
		return nil
	}
	return &s.Videos[0]
}

func (s *Schedule) Find(videoID string) (*Video, bool) {
	for i := range s.Videos {
		if s.Videos[i].ID == videoID {
			return &s.Videos[i], true
		}
	}
	return nil, false
}

func (s *Schedule) Next(videoID string) (*Video, error) {
	for i, v := range s.Videos {
		if v.ID == videoID {
			if i+1 < len(s.Videos) {
				return &s.Videos[i+1], nil
			}
			return nil, fmt.Errorf("no more videos after %q", videoID)
		}
	}
	return nil, fmt.Errorf("video %q not found in schedule", videoID)
}
