package anilist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"vnclub/club"
)

const (
	endpoint  = "https://graphql.anilist.co"
	userAgent = "vnclub/0.1 (+https://github.com/uisan-dev/vnclub)"
)

func NewClient() *Client {
	return &Client{
		http:    &http.Client{Timeout: 15 * time.Second},
		limiter: time.NewTicker(time.Second),
	}
}

func (c *Client) Close() {
	c.limiter.Stop()
}

func (c *Client) MediaByID(id string) (club.Media, error) {
	n, err := strconv.Atoi(id)
	if err != nil {
		return club.Media{}, fmt.Errorf("%s: %w", id, ErrNotFound)
	}

	data, err := c.query(mediaQuery, map[string]any{"id": n})
	if err != nil {
		return club.Media{}, err
	}

	if data.Media == nil {
		return club.Media{}, fmt.Errorf("%s: %w", id, ErrNotFound)
	}

	return data.Media.toClub(), nil
}

func (c *Client) Search(term string, limit int) ([]club.Media, error) {
	if limit <= 0 || limit > 25 {
		limit = 10
	}

	data, err := c.query(searchQuery, map[string]any{
		"search":  term,
		"perPage": limit,
	})
	if err != nil {
		return nil, err
	}

	if data.Page == nil {
		return nil, nil
	}

	out := make([]club.Media, 0, len(data.Page.Media))
	for _, m := range data.Page.Media {
		out = append(out, m.toClub())
	}
	return out, nil
}

func (c *Client) query(query string, vars map[string]any) (*responseData, error) {
	<-c.limiter.C

	reqBody, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimit
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "json") {
		return nil, fmt.Errorf("expected JSON from anilist, got %s: %s", ct, truncate(string(respBody), 512))
	}

	var parsed graphQLResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parsing GraphQL response: %w (body: %s)", err, truncate(string(respBody), 512))
	}

	if len(parsed.Errors) > 0 {
		msgs := []string{}
		for _, e := range parsed.Errors {
			if strings.Contains(e.Message, "not found") {
				return nil, ErrNotFound
			}
			msgs = append(msgs, e.Message)

		}
		log.Println(msgs)
		return nil, fmt.Errorf("GraphQL: %s", strings.Join(msgs, ", "))
	}

	if parsed.Data == nil {
		return nil, fmt.Errorf("no data from AniList")
	}

	return parsed.Data, nil
}
