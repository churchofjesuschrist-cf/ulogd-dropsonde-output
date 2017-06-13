package plugin

import (
	"log"
	"strconv"

	"github.com/cloudfoundry/dropsonde/logs"
)

type dsMode int

const (
	appUnknown dsMode = -1
	appNone    dsMode = 0
	appOut     dsMode = 1
	appErr     dsMode = 2
)

type dropsondeWriter struct {
	guid     string
	mode     dsMode
	sender   string
	instance uint
	buf      []byte
}

func MakeDsondeWriter() *dropsondeWriter {
	return &dropsondeWriter{mode: appUnknown}
}

func (dw *dropsondeWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	err = nil

	// log.Printf("dropsondeWriter appending %q", p)
	dw.buf = append(dw.buf, p...)

	return
}

func (dw *dropsondeWriter) Flush() {
	defer func() { dw.buf = nil }()

	switch dw.mode {
	case appUnknown:
		log.Printf("Unknown mode for dsonde; requires either {{dsonde \"out\"}} or {{dsonde \"err\"}}")
		fallthrough
	case appNone:
		// log.Printf("no output sent to dsonde")
	case appOut:
		err := logs.SendAppLog(dw.guid, string(dw.buf), dw.sender, strconv.FormatUint(uint64(dw.instance), 10))
		if err != nil {
			log.Printf("sending app log: %v", err)
			return
		}
		// log.Printf("output sent to dsonde")
	case appErr:
		err := logs.SendAppErrorLog(dw.guid, string(dw.buf), dw.sender, strconv.FormatUint(uint64(dw.instance), 10))
		if err != nil {
			log.Printf("sending app error log: %v", err)
			return
		}
		// log.Printf("error sent to dsonde")
	}
	return
}
