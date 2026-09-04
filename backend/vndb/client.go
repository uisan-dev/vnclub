package vndb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"vnclub/club"
)

func (c *Client) Close() {
	c.limiter.Stop()
}

const vnFields = `id, title, alttitle, aliases, olang, devstatus, released,
languages, platforms, length, length_minutes, length_votes, description,
rating, votecount,
image{id, url, thumbnail, sexual, violence},
developers{id, name, original, aliases, type}`

const baseURL string = "https://api.vndb.org/kana"
const userAgent string = "vnclub/0.1 (+https://github.com/uisan-dev/vnclub)"

func (c *Client) MediaByID(id string) (club.Media, error) {
	results, err := c.query(vndbRequest{
		Filters: []any{"id", "=", id},
		Fields:  vnFields,
		Results: 1,
	})
	if err != nil {
		return club.Media{}, err
	}

	if len(results) == 0 {
		return club.Media{}, ErrNotFound
	}

	return results[0].toMedia(), nil
}

func (c *Client) Search(term string, limit int) ([]club.Media, error) {
	if limit <= 0 || limit > 25 {
		limit = 10
	}

	results, err := c.query(vndbRequest{
		Filters: []any{"search", "=", term},
		Fields:  vnFields,
		Sort:    "searchrank",
		Results: limit,
	})
	if err != nil {
		return nil, err
	}

	out := make([]club.Media, 0, len(results))
	for _, r := range results {
		out = append(out, r.toMedia())
	}

	return out, nil
}

func (c *Client) query(req vndbRequest) ([]vndbEntry, error) {
	<-c.limiter.C

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/vn", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, ErrRateLimit
		}
		return nil, fmt.Errorf("%s: %s", resp.Status, string(detail))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed vndbResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parsing response: %w (body: %s)", err, respBody)
	}

	return parsed.Results, nil
}
