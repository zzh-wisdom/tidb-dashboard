// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package region

// StatTag is a tag for statistics of different dimensions.
type StatTag int

const (
	// Integration is The overall value of all other dimension statistics.
	Integration StatTag = iota
	// WrittenBytes is the size of the data written per minute.
	WrittenBytes
	// ReadBytes is the size of the data read per minute.
	ReadBytes
	// WrittenKeys is the number of keys written to the data per minute.
	WrittenKeys
	// ReadKeys is the number of keys read to the data per minute.
	ReadKeys
)

// IntoTag converts a string into a StatTag.
func IntoTag(typ string) StatTag {
	switch typ {
	case "":
		return Integration
	case "integration":
		return Integration
	case "written_bytes":
		return WrittenBytes
	case "read_bytes":
		return ReadBytes
	case "written_keys":
		return WrittenKeys
	case "read_keys":
		return ReadKeys
	default:
		return WrittenBytes
	}
}

func (tag StatTag) String() string {
	switch tag {
	case Integration:
		return "integration"
	case WrittenBytes:
		return "written_bytes"
	case ReadBytes:
		return "read_bytes"
	case WrittenKeys:
		return "written_keys"
	case ReadKeys:
		return "read_keys"
	default:
		panic("unreachable")
	}
}

var Tags = []StatTag{Integration, WrittenBytes, ReadBytes, WrittenKeys, ReadKeys}

// GetDisplayTags returns the actual order of the ResponseTags under the specified baseTag.
func GetDisplayTags(baseTag StatTag) []string {
	displayTags := make([]string, len(Tags))
	for i, tag := range Tags {
		displayTags[i] = tag.String()
		if tag == baseTag {
			displayTags[0], displayTags[i] = displayTags[i], displayTags[0]
		}
	}
	return displayTags
}
