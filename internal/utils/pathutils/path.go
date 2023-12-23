package pathutils

import (
	"errors"
	"strings"
)

// GetPathSegments returns the segments of pth, adjacent '/' characters are treated as a single '/' character.
func GetPathSegments(pth string) []string {
	split := strings.Split(string(pth), "/")
	var segments []string

	for _, segment := range split {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

// ForEachAbsolutePathSegment calls fn for each segment of pth, adjacent '/' characters are treated as a single '/' character.
func ForEachAbsolutePathSegment(pth string, fn func(segment string) error) error {
	if pth == "" {
		panic(errors.New("empty path"))
	}
	if pth[0] != '/' {
		panic(errors.New("path is not absolute"))
	}
	segmentStart := 1

	for i := 1; i < len(pth); i++ {
		if pth[i] == '/' {
			if segmentStart != i {
				err := fn(pth[segmentStart:i])
				if err != nil {
					return err
				}
			}
			segmentStart = i + 1
		}
	}

	if segmentStart < len(pth) {
		return fn(pth[segmentStart:])
	}
	return nil
}

func GetLastPathSegment(pth string) string {
	segments := GetPathSegments(pth)
	return segments[len(segments)-1]
}
