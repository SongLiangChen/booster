package booster

import (
	"github.com/gin-gonic/gin"
	"net/url"
)

type Helper struct {
	c *gin.Context
}

func NewHelper(c *gin.Context) *Helper {
	return &Helper{c: c}
}

func (h *Helper) GetContext() *gin.Context {
	return h.c
}

func (h *Helper) GetParams() map[string]string{
	r := h.GetContext().Request
	r.ParseForm()
	var form url.Values
	if r.Method == "GET" {
		form = r.Form
	}else {
		form = r.PostForm
	}
	params := make(map[string]string)
	for key, val := range form {
		params[key] = val[0]
	}
	return params
}

func (h *Helper) StringParam(param string) string {
	if param == "" {
		return ""
	}
	c := h.GetContext()
	if c.Request.Method == "GET" {
		return c.Query(param)
	}
	return c.PostForm(param)
}

func (h *Helper) StringParamDefault(param string, defaultVal string) string {
	v := h.StringParam(param)
	if v == "" {
		v = defaultVal
	}
	return v
}
