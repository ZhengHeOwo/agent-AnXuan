package analyse

import "errors"

var (
	errPreferencesBucketNotFound = errors.New("preferences bucket not found")

	errPreferenceNotFound = errors.New("preference not found")

	errNoAction = errors.New("NO_ACTION")
)
