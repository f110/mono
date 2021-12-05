package errors

type MemcachedError struct {
	Status  uint16
	Message string
}

var (
	ItemNotFound = &MemcachedError{Status: 1, Message: "Key not found"}
	ItemExists   = &MemcachedError{Status: 2, Message: "Key exists"}
)

func (e *MemcachedError) Error() string {
	return e.Message
}
