package analyse

import "errors"

var (
	errPreferencesDatabaseBucketNotFound = errors.New("preferences database bucket not found")

	errPreferenceNotFound = errors.New("preference not found")

	errNoAction = errors.New("NO_ACTION")
)
