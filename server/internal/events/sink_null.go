package events

type SinkNull struct{}

func newSinkNull() *SinkNull {
	return &SinkNull{}
}

func (s *SinkNull) Send(subject string, event any) error {
	return nil
}

func (s *SinkNull) Close() error {
	return nil
}
