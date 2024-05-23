package pathutils

import (
	"errors"
	"strings"
)

// GetPathSegments returns the segments of pth, adjacent '/' characters are treated as a single '/' character.
func GetPathSegments[T ~string](pth T) []string {
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
// The path is not cleaned, so fn may be invoked with '.' or '..' as segment. The function panics if pth does not start with '/'.
func ForEachAbsolutePathSegment[T ~string](pth T, fn func(segment string, startIndex, endIndex int) error) error {
	if pth != "" && pth[0] != '/' {
		panic(errors.New("path is not absolute"))
	}
	return ForEachPathSegment(pth, fn)
}

// ForEachPathSegment calls fn for each segment of pth, adjacent '/' characters are treated as a single '/' character.
// The path is not cleaned, so fn may be invoked with '.' or '..' as segment.
func ForEachPathSegment[T ~string](pth T, fn func(segment string, startIndex, endIndex int) error) error {
	if pth == "" {
		panic(errors.New("empty path"))
	}
	segmentStart := 1

	for i := 1; i < len(pth); i++ {
		if pth[i] == '/' {
			if segmentStart != i {
				err := fn(string(pth[segmentStart:i]), segmentStart, i)
				if err != nil {
					return err
				}
			}
			segmentStart = i + 1
		}
	}

	if segmentStart < len(pth) {
		return fn(string(pth[segmentStart:]), segmentStart, len(pth))
	}
	return nil
}

func GetLastPathSegment(pth string) string {
	segments := GetPathSegments(pth)
	return segments[len(segments)-1]
}

// ContainsRelativePathSegments reports whether $pth contains '.' or '..' segments.
func ContainsRelativePathSegments[T ~string](pth T) bool {
	yes := false

	if pth == "" {
		panic(errors.New("empty path"))
	}

	err := ForEachPathSegment(pth, func(segment string, startIndex, endIndex int) error {
		if segment == "." || segment == ".." {
			yes = true
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	return yes
}
