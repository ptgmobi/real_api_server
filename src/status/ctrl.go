package status

import (
	"bytes"
	"html/template"
	"sync"
	"time"
)

const (
	SampleNo = iota
	SampleReady
	SampleDoing
	SampleDone
)

var ctrlJsTpl string = `
    <div>
        <h3 id = "msg"></h3>
    </div>

    <script>
        window.onload = function() {
            var wait_sec = {{.WaitSec}};

            var clock = setInterval(function() {
                document.getElementById("msg").innerText =
                "[{{.PhaseContent}}] 倒计时:" + wait_sec + "秒...";
                wait_sec -= 1;
            }, 1000);

            setTimeout(function() {
                clearInterval(clock);
                window.location.reload();
            }, (wait_sec + 1) * 1000);
        }
    </script>
`

type tplInfo struct {
	WaitSec      int
	PhaseContent string
}

type ctrl struct {
	sync.RWMutex
	sampleStatus int
	ticker       *time.Ticker
	remainSec    int
	reset        chan bool
}

var gTemplate *template.Template

func init() {
	var err error
	gTemplate, err = template.New("sample").Parse(ctrlJsTpl)
	if err != nil {
		panic(err)
	}
}

func (c *ctrl) readySample(sec int) {
	c.Lock()
	defer c.Unlock()

	if c.sampleStatus != SampleNo {
		return
	}

	c.sampleStatus = SampleReady
	c.remainSec = sec

	go func() {
		if c.ticker != nil {
			panic("FATAL: readySample: ticker != nil")
		}
		c.ticker = time.NewTicker(time.Second)

		for range c.ticker.C {
			c.Lock()
			c.remainSec -= 1
			if c.remainSec <= 0 {
				c.ticker.Stop()
				c.ticker = nil
				c.Unlock()
				break
			}
			c.Unlock()
		}

		go c.doSample()
	}()
}

func (c *ctrl) doSample() {
	c.Lock()
	defer c.Unlock()

	if c.sampleStatus != SampleReady {
		return
	}
	c.sampleStatus = SampleDoing
	c.remainSec = 60

	go func() {
		if c.ticker != nil {
			panic("FATAL: doingSample: ticker != nil")
		}
		c.ticker = time.NewTicker(time.Second)
		for range c.ticker.C {
			c.Lock()
			c.remainSec -= 1
			if c.remainSec <= 0 {
				c.ticker.Stop()
				c.ticker = nil
				c.Unlock()
				break
			}
			c.Unlock()
		}

		go c.doneSample()
	}()
}

func (c *ctrl) doneSample() {
	c.Lock()
	defer c.Unlock()

	if c.sampleStatus != SampleDoing {
		return
	}
	c.sampleStatus = SampleDone
	c.remainSec = 60 * 5 // 5 min

	go func() {
		if c.ticker != nil {
			panic("FATAL: doneSample: ticker != nil")
		}
		if c.reset != nil {
			panic("FATAL: doneSample: reset channel != nil")
		}
		c.ticker = time.NewTicker(time.Second)
		c.reset = make(chan bool)
		for {
			select {
			case <-c.ticker.C:
				c.Lock()
				c.remainSec -= 1
				if c.remainSec <= 0 {
					close(c.reset)
					c.reset = nil
					c.sampleStatus = SampleNo
					c.ticker.Stop()
					c.ticker = nil
					c.Unlock()
					return
				}
				c.Unlock()

			case <-c.reset:
				c.Lock()
				c.remainSec = 60 * 5
				c.Unlock()
			}
		}
	}()
}

func (c *ctrl) resetSample() {
	if c.reset != nil {
		c.reset <- true
	}
}

func (c *ctrl) getSampleStatus() (ok bool, buf *bytes.Buffer) {
	c.RLock()
	defer c.RUnlock()

	if c.sampleStatus == SampleDone {
		c.resetSample()
		return true, nil
	}

	buf = bytes.NewBuffer(nil)
	var tplParam tplInfo

	switch c.sampleStatus {

	case SampleNo:
		now := time.Now().UTC()
		readySec := 60 - now.Second()
		tplParam.PhaseContent = "采样初始化"
		tplParam.WaitSec = readySec
		go c.readySample(readySec)

	case SampleReady:
		tplParam.PhaseContent = "采样初始化"
		tplParam.WaitSec = c.remainSec

	case SampleDoing:
		tplParam.PhaseContent = "正在采样中"
		tplParam.WaitSec = c.remainSec

	default:
		panic("untouched code here")
	}

	if err := gTemplate.Execute(buf, &tplParam); err != nil {
		panic(err)
	}

	return false, buf
}

func (c *ctrl) SampleSwitch() bool {
	c.RLock()
	defer c.RUnlock()
	return c.sampleStatus != SampleNo
}
