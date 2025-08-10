// +build test

package internal

// AddMockType is only for testing - provides mock data for test scenarios
func (rs *ReadState) AddMockType(typeName string) {
	if rs.data[typeName] == nil {
		rs.data[typeName] = []interface{}{"mock_data"}
	}
}

// NewTraditionalSerializerForTests creates a traditional serializer for test use
func NewTraditionalSerializerForTests() *TraditionalSerializer {
	return NewTraditionalSerializer()
}
