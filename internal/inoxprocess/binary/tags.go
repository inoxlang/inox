package binary

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type tag struct {
	Name string `json:"name"`
}

func FetchTags() (tags []tag, _ error) {

	resp, err := http.Get(TAGS_API_ENDPOINT)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", TAGS_API_ENDPOINT, err)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read body of %s: %w", TAGS_API_ENDPOINT, err)
	}

	err = json.Unmarshal(body, &tags)

	if err != nil {
		return nil, err
	}

	return
}
