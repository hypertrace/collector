package urlencoded

import (
	"fmt"
	"github.com/hypertrace/collector/processors"
	"net/url"

	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
)

var _ filters.Filter = (*urlEncodedFilter)(nil)

type urlEncodedFilter struct {
	m *regexmatcher.Matcher
}

func NewFilter(m *regexmatcher.Matcher) filters.Filter {
	return &urlEncodedFilter{m}
}

const urlAttributeStr = "http.url"

func (f *urlEncodedFilter) Name() string {
	return "urlencoded"
}

func (f *urlEncodedFilter) RedactAttribute(key string, value pdata.AttributeValue) (*processors.ParsedAttribute, error) {
	if len(value.StringVal()) == 0 {
		return nil, nil
	}

	var u *url.URL
	var err error

	rawString := value.StringVal()
	isURLAttr := key == urlAttributeStr
	if isURLAttr {
		u, err = url.Parse(value.StringVal())
		if err != nil {
			return nil, filters.WrapError(filters.ErrUnprocessableValue, err.Error())
		}
		rawString = u.RawQuery
	}

	params, err := url.ParseQuery(rawString)
	if err != nil {
		return nil, filters.WrapError(filters.ErrUnprocessableValue, err.Error())
	}

	v := url.Values{}
	attr := &processors.ParsedAttribute{
		Redacted:  map[string]string{},
		Flattened: map[string]string{},
	}
	for param, values := range params {
		fqn := fmt.Sprintf("%s.%s", key, param)
		for idx, value := range values {
			attr.Flattened[fqn] = value
			path := param
			if !isURLAttr {
				if len(values) > 1 {
					path = fmt.Sprintf("$.%s[%d]", param, idx)
				} else {
					path = fmt.Sprintf("$.%s", param)
				}
			}

			if isRedactedByKey, isSession, redactedValue := f.m.FilterKeyRegexs(param, key, value, path); isRedactedByKey {
				if isSession {
					// TODO
				}
				attr.Redacted[fqn] = value
				v.Add(param, redactedValue)
			} else if isRedactedByValue, redactedValue := f.m.FilterStringValueRegexs(value, key, path); isRedactedByValue {
				attr.Redacted[fqn] = value
				v.Add(param, redactedValue)
			} else {
				v.Add(param, value)
			}
		}
	}

	if len(attr.Redacted) > 0 {
		encoded := v.Encode()
		if isURLAttr {
			u.RawQuery = encoded
			value.SetStringVal(u.String())
		} else {
			value.SetStringVal(encoded)
		}
	}

	return attr, nil
}
