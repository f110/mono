package main

import (
	"bytes"
	"testing"
)

func TestConfig_ToDNSControl(t *testing.T) {
	conf, err := ReadConfig("testdata/records.yaml")
	if err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	for _, v := range conf {
		buf.WriteString(v.ToDNSControl())
	}
	got := buf.String()
	want := `D('f110.dev', REG_NONE, DnsProvider(DNS_GCLOUD),
    NAMESERVER_TTL('6h'),
    DefaultTTL('300s'),
    A('test', '127.0.0.1', TTL('600'))
);
D('f110.jp', REG_NONE, DnsProvider(DNS_GCLOUD),
    A('test', '127.0.0.1'),
    A('m', '127.0.0.1', TTL('300')),
    A('m', '127.0.0.2', TTL('300'))
);
`

	if got != want {
		t.Log(got)
		t.Error("Unexpected result")
	}
}
