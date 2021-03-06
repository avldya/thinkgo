// Copyright 2016 HenryLee. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package thinkgo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/henrylee2cn/thinkgo/acceptencoder"
	"github.com/henrylee2cn/thinkgo/apiware"
	"github.com/henrylee2cn/thinkgo/logging"
	"github.com/henrylee2cn/thinkgo/utils"
)

type (
	// GlobalVariables defines the global frames, configuration, function and so on.
	GlobalVariables struct {
		// the list of applications that have been created.
		frames map[string]*Framework
		// global config
		config GlobalConfig
		// Error replies to the request with the specified error message and HTTP code.
		// It does not otherwise end the request; the caller should ensure no further
		// writes are done to response.
		// The error message should be plain text.
		errorFunc ErrorFunc
		// The following is only for the APIHandler
		binderrorFunc BinderrorFunc
		// Decode params from request body.
		bodydecoder apiware.Bodydecoder
		// When the APIHander's parameter name (struct tag) is unsetted,
		// it is mapped from the structure field name by default.
		// If `paramNameMapper` is nil, use snake style.
		// If the APIHander's parameter binding fails, the default handler is invoked
		paramNameMapper apiware.ParamNameMapper
		// global file cache system manager
		fsManager *FileServerManager
		// Render is a custom thinkgo template renderer using pongo2.
		render *Render

		// The path for the upload files.
		// When does not have a custom route, the route is automatically created.
		upload PresetStatic

		// The path for the static files.
		// When does not have a custom route, the route is automatically created.
		static PresetStatic

		// The path for the log files
		logDir string

		syslog *logging.Logger
		bizlog *logging.Logger
	}
	// PresetStatic is the system default static file routing information
	PresetStatic struct {
		root       string
		nocompress bool
		nocache    bool
		handlers   []Handler
	}
)

var (
	// global is the global configuration, functions and so on.
	global = func() *GlobalVariables {
		global := &GlobalVariables{
			frames:          make(map[string]*Framework),
			config:          globalConfig,
			errorFunc:       defaultErrorFunc,
			bodydecoder:     defaultBodyJSONFunc,
			binderrorFunc:   defaultBinderrorFunc,
			paramNameMapper: defaultParamNameMapper,
			fsManager: newFileServerManager(
				globalConfig.Cache.SizeMB*1024*1024,
				globalConfig.Cache.Expire,
				globalConfig.Cache.Enable,
				globalConfig.Gzip.Enable,
			),
			upload: defaultUpload,
			static: defaultStatic,
			logDir: defaultLogDir,
		}
		if globalConfig.Cache.Enable {
			global.render = newRender(func(name string) (http.File, error) {
				return global.fsManager.Open(name, "", false)
			})
		} else {
			global.render = newRender(nil)
		}
		global.initLogger()
		return global
	}()
	defaultErrorFunc = func(ctx *Context, errStr string, status int) {
		ctx.Log().Error(errStr)
		statusText := http.StatusText(status)
		if len(errStr) > 0 {
			errStr = `<br><p><b style="color:red;">[ERROR]</b> <pre>` + errStr + `</pre></p>`
		}
		ctx.W.Header().Set(HeaderXContentTypeOptions, nosniff)
		ctx.HTML(status, fmt.Sprintf("<html>\n"+
			"<head><title>%d %s</title></head>\n"+
			"<body bgcolor=\"white\">\n"+
			"<center><h1>%d %s</h1></center>\n"+
			"<hr>\n<center>thinkgo/%s</center>\n%s\n</body>\n</html>\n",
			status, statusText, status, statusText, VERSION, errStr),
		)
	}
	// The default body decoder is json format decoding
	defaultBodyJSONFunc = func(dest reflect.Value, body []byte) error {
		var err error
		if dest.Kind() == reflect.Ptr {
			err = json.Unmarshal(body, dest.Interface())
		} else {
			err = json.Unmarshal(body, dest.Addr().Interface())
		}
		return err
	}
	defaultBinderrorFunc = func(ctx *Context, err error) {
		ctx.String(http.StatusBadRequest, "%v", err)
	}
	defaultParamNameMapper = utils.SnakeString
	// The default path for the upload files
	defaultUpload = PresetStatic{
		root: "./upload/",
	}
	// The default path for the static files
	defaultStatic = PresetStatic{
		root: "./static/",
	}
	// The default path for the log files
	defaultLogDir = "./log/"
)

func init() {
	fmt.Println(banner[1:])
	global.syslog.Criticalf("The PID of the current process is %d", os.Getpid())
	if global.config.warnMsg != "" {
		Warning(global.config.warnMsg)
		global.config.warnMsg = ""
	}
	// init file cache
	acceptencoder.InitGzip(global.config.Gzip.MinLength, global.config.Gzip.CompressLevel, global.config.Gzip.Methods)
}

// AllFrames returns the list of applications that have been created.
func AllFrames() map[string]*Framework {
	return global.frames
}

// GetFrame returns the specified frame instance by name.
func GetFrame(name string) (*Framework, bool) {
	frame, ok := global.frames[name]
	return frame, ok
}

// Close closes all the frame services.
// TODO: close listeners
func Close() {
	for _, frame := range global.frames {
		frame.Close()
	}
	CloseLog()
}

// HandleError calls the default error handler.
func HandleError(ctx *Context, errStr string, status int) {
	global.errorFunc(ctx, errStr, status)
}

// SetErrorFunc sets the global default `ErrorFunc` function.
func SetErrorFunc(errorFunc ErrorFunc) {
	global.errorFunc = errorFunc
}

// DecodeBody decodes params from request body.
func DecodeBody(dest reflect.Value, body []byte) error {
	return global.bodydecoder(dest, body)
}

// SetBodydecoder sets the global default `Bodydecoder` function.
func SetBodydecoder(bodydecoder apiware.Bodydecoder) {
	global.bodydecoder = bodydecoder
}

// HandleBinderror calls the default parameter binding failure handler.
func HandleBinderror(ctx *Context, err error) {
	global.binderrorFunc(ctx, err)
}

// SetBinderrorFunc sets the global default `BinderrorFunc` function.
func SetBinderrorFunc(binderrorFunc BinderrorFunc) {
	global.binderrorFunc = binderrorFunc
}

// MapParamName maps the APIHander's parameter name from the structure field.
// When the APIHander's parameter name (struct tag) is unsetted,
// it is mapped from the structure field name by default.
// If `paramNameMapper` is nil, use snake style.
func MapParamName(fieldName string) (paramName string) {
	return global.paramNameMapper(fieldName)
}

// SetParamNameMapper sets the global default `ParamNameMapper` function.
func SetParamNameMapper(paramNameMapper apiware.ParamNameMapper) {
	global.paramNameMapper = paramNameMapper
}

// GetRender returns a custom thinkgo template renderer using pongo2.
func GetRender() *Render {
	return global.render
}

// RenderVar sets the global template variable or function for pongo2 render.
func RenderVar(name string, v interface{}) {
	global.render.TemplateVar(name, v)
}

// LogDir returns logs folder path with a slash at the end
func LogDir() string {
	return global.logDir
}

// SetUpload sets upload folder path such as `./upload/`
// with a slash `/` at the end.
// note: it should be called before Run()
func SetUpload(dir string, nocompress bool, nocache bool, handlers ...Handler) {
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	global.upload = PresetStatic{
		root:       dir,
		nocompress: nocompress,
		nocache:    nocache,
		handlers:   handlers,
	}
}

// UploadDir returns upload folder path with a slash at the end
func UploadDir() string {
	return global.upload.root
}

// SetStatic sets static folder path, such as `./staic/`
// with a slash `/` at the end.
// note: it should be called before Run()
func SetStatic(dir string, nocompress bool, nocache bool, handlers ...Handler) {
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	global.static = PresetStatic{
		root:       dir,
		nocompress: nocompress,
		nocache:    nocache,
		handlers:   handlers,
	}
}

// StaticDir returns static folder path with a slash at the end
func StaticDir() string {
	return global.static.root
}

// Log returns the global logger used by the user bissness.
func Log() {
	global.bizlog.Close()
}

// CloseLog closes global loggers.
func CloseLog() {
	global.bizlog.Close()
	global.syslog.Close()
}

// Print logs a system message using CRITICAL as log level.
func Print(args ...interface{}) {
	global.syslog.Critical(args...)
}

// Printf logs a system message using CRITICAL as log level.
func Printf(format string, args ...interface{}) {
	global.syslog.Criticalf(format, args...)
}

// Fatal is equivalent to l.Critical(fmt.Sprint()) followed by a call to os.Exit(1).
func Fatal(args ...interface{}) {
	global.bizlog.Fatal(args...)
}

// Fatalf is equivalent to l.Critical followed by a call to os.Exit(1).
func Fatalf(format string, args ...interface{}) {
	global.bizlog.Fatalf(format, args...)
}

// Panic is equivalent to l.Critical(fmt.Sprint()) followed by a call to panic().
func Panic(args ...interface{}) {
	global.bizlog.Panic(args...)
}

// Panicf is equivalent to l.Critical followed by a call to panic().
func Panicf(format string, args ...interface{}) {
	global.bizlog.Panicf(format, args...)
}

// Critical logs a message using CRITICAL as log level.
func Critical(args ...interface{}) {
	global.bizlog.Critical(args...)
}

// Criticalf logs a message using CRITICAL as log level.
func Criticalf(format string, args ...interface{}) {
	global.bizlog.Criticalf(format, args...)
}

// Error logs a message using ERROR as log level.
func Error(args ...interface{}) {
	global.bizlog.Error(args...)
}

// Errorf logs a message using ERROR as log level.
func Errorf(format string, args ...interface{}) {
	global.bizlog.Errorf(format, args...)
}

// Warning logs a message using WARNING as log level.
func Warning(args ...interface{}) {
	global.bizlog.Warning(args...)
}

// Warningf logs a message using WARNING as log level.
func Warningf(format string, args ...interface{}) {
	global.bizlog.Warningf(format, args...)
}

// Notice logs a message using NOTICE as log level.
func Notice(args ...interface{}) {
	global.bizlog.Notice(args...)
}

// Noticef logs a message using NOTICE as log level.
func Noticef(format string, args ...interface{}) {
	global.bizlog.Noticef(format, args...)
}

// Info logs a message using INFO as log level.
func Info(args ...interface{}) {
	global.bizlog.Info(args...)
}

// Infof logs a message using INFO as log level.
func Infof(format string, args ...interface{}) {
	global.bizlog.Infof(format, args...)
}

// Debug logs a message using DEBUG as log level.
func Debug(args ...interface{}) {
	global.bizlog.Debug(args...)
}

// Debugf logs a message using DEBUG as log level.
func Debugf(format string, args ...interface{}) {
	global.bizlog.Debugf(format, args...)
}
