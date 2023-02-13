package action

import (
	"html/template"
	"net/http"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	sv "github.com/webtor-io/web-ui-v2/services"
	j "github.com/webtor-io/web-ui-v2/services/job"
	w "github.com/webtor-io/web-ui-v2/services/web"
)

type PostArgs struct {
	ResourceID string
	ItemID     string
	Claims     *sv.Claims
}

type PostData struct {
	w.ErrorData
	Job  *sv.Job
	Args *PostArgs
}

type Handler struct {
	*w.TemplateHandler
	*w.ClaimsHandler
	jobs *j.Handler
}

func NewHandler(c *cli.Context, jobs *j.Handler) *Handler {
	return &Handler{
		TemplateHandler: w.NewTemplateHandler(c),
		ClaimsHandler:   w.NewClaimsHandler(),
		jobs:            jobs,
	}
}

func (s *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/download-file", func(c *gin.Context) {
		s.post(c, "download")
	})
	r.POST("/download-dir", func(c *gin.Context) {
		s.post(c, "download")
	})
	r.POST("/preview-image", func(c *gin.Context) {
		s.post(c, "preview-image")
	})
}

func (s *Handler) RegisterTemplates(r multitemplate.Renderer) {
	s.RegisterTemplate(
		r,
		"action/post",
		[]string{"async"},
		[]string{},
		template.FuncMap{},
	)
}

func (s *Handler) bindPostArgs(c *gin.Context) (*PostArgs, error) {
	rID, ok := c.GetPostFormArray("resource-id")
	if !ok {
		return nil, errors.Errorf("no resource id provided")
	}
	iID, ok := c.GetPostFormArray("item-id")
	if !ok {
		return nil, errors.Errorf("no item id provided")
	}
	return &PostArgs{
		ResourceID: rID[0],
		ItemID:     iID[0],
		Claims:     s.MakeClaims(c),
	}, nil
}

func (s *Handler) post(c *gin.Context, action string) {
	var (
		d    PostData
		err  error
		args *PostArgs
		job  *sv.Job
	)
	index := s.MakeTemplate(c, "action/post", &d)
	args, err = s.bindPostArgs(c)
	if err != nil {
		d.Err = errors.Wrap(err, "wrong args provided")
		index.R(http.StatusBadRequest)
		return
	}
	d.Args = args
	job, err = s.jobs.Action(args.Claims, args.ResourceID, args.ItemID, action)
	if err != nil {
		d.Err = errors.Wrap(err, "failed to start downloading")
		index.R(http.StatusBadRequest)
		return
	}
	d.Job = job
	index.R(http.StatusOK)
}