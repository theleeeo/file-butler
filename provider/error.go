package provider

// import "errors"

// var (
// 	NewErrNotFound = func(desc string) error {
// 		return (&ProviderError{Err: errors.New("not found")}).WithCode(404).WithDescription(desc)
// 	}

// 	NewErrDenied = func(desc string) error {
// 		return (&ProviderError{Err: errors.New("access denied")}).WithCode(403).WithDescription(desc)
// 	}
// )

// type ProviderError struct {
// 	Code int
// 	Err  error
// 	Desc string
// }

// func (e *ProviderError) Error() string {
// 	return e.Err.Error()
// }

// func (e *ProviderError) Unwrap() error {
// 	return e.Err
// }

// func (e *ProviderError) WithCode(code int) *ProviderError {
// 	e.Code = code
// 	return e
// }

// func (e *ProviderError) WithDescription(desc string) *ProviderError {
// 	e.Desc = desc
// 	return e
// }
