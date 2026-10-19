package inventory

import "errors"

type RuntimeMetadataError struct{ Detail string }

func (e *RuntimeMetadataError) Error() string { return e.Detail }
func RuntimeMetadataUnverified(err error) bool {
	var target *RuntimeMetadataError
	return errors.As(err, &target)
}
