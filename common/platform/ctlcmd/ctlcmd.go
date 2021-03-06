package ctlcmd

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"fmt"

	"v2ray.com/core/common/buf"
	"v2ray.com/core/common/platform"
)

//go:generate go run v2ray.com/core/common/errors/errorgen
// 还要依赖v2ctl可执行程序 ? 
// args: [config, /Users/demon/Desktop/work/gowork/src/v2ray.com/core/main/config.json]
// input: os.Stdin
func Run(args []string, input io.Reader) (buf.MultiBuffer, error) {
	// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/main/v2ctl
	v2ctl := platform.GetToolLocation("v2ctl") 
	fmt.Printf("v2ctrl:%s\n", v2ctl)

	
	if _, err := os.Stat(v2ctl); err != nil {
		return nil, newError("v2ctl doesn't exist").Base(err)
	}

	var errBuffer buf.MultiBufferContainer
	var outBuffer buf.MultiBufferContainer

	cmd := exec.Command(v2ctl, args...)
	cmd.Stderr = &errBuffer
	cmd.Stdout = &outBuffer
	cmd.SysProcAttr = getSysProcAttr()
	if input != nil {
		cmd.Stdin = input
	}

	if err := cmd.Start(); err != nil {
		return nil, newError("failed to start v2ctl").Base(err)
	}

	if err := cmd.Wait(); err != nil {
		msg := "failed to execute v2ctl"
		if errBuffer.Len() > 0 {
			msg += ": \n" + strings.TrimSpace(errBuffer.MultiBuffer.String())
		}
		return nil, newError(msg).Base(err)
	}

	// log stderr, info message
	if !errBuffer.IsEmpty() {
		newError("<v2ctl message> \n", strings.TrimSpace(errBuffer.MultiBuffer.String())).AtInfo().WriteToLog()
	}

	return outBuffer.MultiBuffer, nil
}
