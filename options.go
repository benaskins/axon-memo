package memo

import fact "github.com/benaskins/axon-fact"

// Option configures a Server during construction.
type Option func(*Server)

// WithAnalytics sets the analytics event emitter.
func WithAnalytics(a AnalyticsEmitter) Option {
	return func(s *Server) {
		s.analytics = a
	}
}

// WithEventStore sets the event store for domain events.
func WithEventStore(es fact.EventStore) Option {
	return func(s *Server) {
		s.eventStore = es
	}
}
