package core

import (
	"crypto/rand"
	"encoding/json"
)

var marshalIndentJSON = json.MarshalIndent

var randRead = rand.Read
