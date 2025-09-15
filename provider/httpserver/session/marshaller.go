package session

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
)

type Marshaller interface {
	MarshalSession(session *SessionData) ([]byte, error)
	UnmarshalSession(data []byte) (*SessionData, error)
}

type gobMarshaller struct{}

func NewGobMarshaller() Marshaller {
	return &gobMarshaller{}
}

// MarshalSession use gob to marshal session
func (g *gobMarshaller) MarshalSession(session *SessionData) ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	if err := enc.Encode(session); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// UnmarshalSession use gob to unmarshal session
func (g *gobMarshaller) UnmarshalSession(data []byte) (*SessionData, error) {
	var session SessionData
	dec := gob.NewDecoder(bytes.NewBuffer(data))
	if err := dec.Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

type jsonMarshaller struct{}

func NewJSONMarshaller() Marshaller {
	return &jsonMarshaller{}
}

// MarshalSession use gob to marshal session
func (g *jsonMarshaller) MarshalSession(session *SessionData) ([]byte, error) {
	return json.Marshal(session)
}

// UnmarshalSession use gob to unmarshal session
func (g *jsonMarshaller) UnmarshalSession(data []byte) (*SessionData, error) {
	result := &SessionData{}
	if err := json.Unmarshal(data, result); err != nil {
		return nil, err
	}
	return result, nil
}

func init() {
	// register type to be used with session data
	gob.Register(&SessionData{})
	gob.Register(map[string]interface{}{})
	gob.Register(map[string]int{})
	gob.Register(map[string]string{})
	gob.Register([]string{})
	gob.Register([]int{})
	gob.Register([]interface{}{})
}
