package booster

type envelope struct {
	t       int
	userIds []string
	msg     []byte
	filter  FilterFunc
}
