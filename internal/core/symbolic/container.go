package symbolic

import "errors"

var ErrCannotAddNonSharableToSharedContainer = errors.New("cannot add a non sharable element to a shared container")
