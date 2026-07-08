package mqtt

import (
	"strings"
	"unicode/utf8"
)

// Topic Names and Topic Filters
// The MQTT v3.1.1 spec clarifies a number of ambiguities with regard
// to the validity of Topic strings.
// - A Topic must be between 1 and 65535 bytes.
// - A Topic is case sensitive.
// - A Topic may contain whitespace.
// - A Topic containing a leading forward slash is different than a Topic without.
// - A Topic may be "/" (two levels, both empty string).
// - A Topic must be UTF-8 encoded.
// - A Topic may contain any number of levels.
// - A Topic may contain an empty level (two forward slashes in a row).
// - A TopicName may not contain a wildcard.
// - A TopicFilter may only have a # (multi-level) wildcard as the last level.
// - A TopicFilter may contain any number of + (single-level) wildcards.
// - A TopicFilter with a # will match the absence of a level
//     Example:  a subscription to "foo/#" will match messages published to "foo".

// ValidUTF8String reports whether s is a well-formed MQTT UTF-8 string:
// valid UTF-8 that contains no U+0000 [MQTT-1.5.3-1, MQTT-1.5.3-2].
func ValidUTF8String(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

// ValidatePattern checks that pattern is a valid MQTT topic filter:
// 1-65535 bytes of well-formed UTF-8 [MQTT-4.7.3-1..3], '#' only as the
// final entire level [MQTT-4.7.1-2] and '+' only as an entire level
// [MQTT-4.7.1-3].
func ValidatePattern(pattern string) error {
	if len(pattern) == 0 {
		return ErrInvalidTopicEmptyString
	}
	if len(pattern) > maxFieldLength {
		return ErrInvalidTopicTooLong
	}
	if !ValidUTF8String(pattern) {
		return ErrInvalidTopicUTF8
	}
	levels := strings.Split(pattern, "/")
	for i, level := range levels {
		if strings.Contains(level, "#") {
			if level != "#" || i != len(levels)-1 {
				return ErrInvalidTopicMultilevel
			}
		}
		if strings.Contains(level, "+") && level != "+" {
			return ErrInvalidTopicSinglelevel
		}
	}
	return nil
}

// ValidateTopic checks that topic is a valid MQTT topic name: a valid
// pattern that contains no wildcard characters [MQTT-3.3.2-2].
func ValidateTopic(topic string) error {
	if strings.Contains(topic, "#") || strings.Contains(topic, "+") {
		return ErrInvalidWildcardTopic
	}
	return ValidatePattern(topic)
}
