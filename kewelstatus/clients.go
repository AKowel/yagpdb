package kewelstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type QueueItem struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Year          int      `json:"year,omitempty"`
	Subtitle      string   `json:"subtitle,omitempty"`
	Size          float64  `json:"size"`
	Progress      float64  `json:"progress"`
	DLSpeed       float64  `json:"dlspeed"`
	ETA           float64  `json:"eta"`
	Status        string   `json:"status"`
	TrackedState  string   `json:"trackedState"`
	TrackedStatus string   `json:"trackedStatus"`
	QbitState     string   `json:"qbitState,omitempty"`
	Seeds         int      `json:"seeds,omitempty"`
	Messages      []string `json:"messages"`
}

type QueueErrors struct {
	Qbit   *string `json:"qbit"`
	Radarr *string `json:"radarr"`
	Sonarr *string `json:"sonarr"`
}

type QueueData struct {
	Movies          []QueueItem `json:"movies"`
	Shows           []QueueItem `json:"shows"`
	QbitConnected   bool        `json:"qbitConnected"`
	RadarrReachable bool        `json:"radarrReachable"`
	SonarrReachable bool        `json:"sonarrReachable"`
	Errors          QueueErrors `json:"errors"`
}

// FetchQueue calls the kewel-queue API's /api/queue endpoint.
func FetchQueue(queueAPIURL string) (*QueueData, error) {
	if queueAPIURL == "" {
		return nil, fmt.Errorf("no queue API URL configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queueAPIURL+"/api/queue", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("queue API %d", resp.StatusCode)
	}

	var data QueueData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

type WantedMovie struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

// FetchWantedMovies calls Radarr's wanted/missing endpoint. Mirrors the Node bot:
// failures are swallowed and an empty slice returned, never an error.
func FetchWantedMovies(radarrURL, apiKey string) []WantedMovie {
	if radarrURL == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v3/wanted/missing?apikey=%s&pageSize=25&sortKey=releaseDate&sortDirection=descending", radarrURL, apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}

	var data struct {
		Records []WantedMovie `json:"records"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	return data.Records
}

type WantedEpisode struct {
	ID            int    `json:"id"`
	SeriesTitle   string `json:"seriesTitle"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
}

// FetchWantedEpisodes calls Sonarr's wanted/missing endpoint. Mirrors the Node bot:
// failures are swallowed and an empty slice returned, never an error.
func FetchWantedEpisodes(sonarrURL, apiKey string) []WantedEpisode {
	if sonarrURL == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v3/wanted/missing?apikey=%s&pageSize=25&sortKey=airDateUtc&sortDirection=descending", sonarrURL, apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}

	var data struct {
		Records []struct {
			ID     int `json:"id"`
			Series struct {
				Title string `json:"title"`
			} `json:"series"`
			SeriesTitle   string `json:"seriesTitle"`
			SeasonNumber  int    `json:"seasonNumber"`
			EpisodeNumber int    `json:"episodeNumber"`
		} `json:"records"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}

	episodes := make([]WantedEpisode, 0, len(data.Records))
	for _, r := range data.Records {
		title := r.Series.Title
		if title == "" {
			title = r.SeriesTitle
		}
		if title == "" {
			title = "Unknown"
		}
		episodes = append(episodes, WantedEpisode{
			ID:            r.ID,
			SeriesTitle:   title,
			SeasonNumber:  r.SeasonNumber,
			EpisodeNumber: r.EpisodeNumber,
		})
	}
	return episodes
}
