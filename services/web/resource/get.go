package resource

import (
	"context"
	csrf "github.com/utrack/gin-csrf"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	ra "github.com/webtor-io/rest-api/services"
	sv "github.com/webtor-io/web-ui-v2/services"
	sw "github.com/webtor-io/web-ui-v2/services/web"
)

var (
	sampleReg = regexp.MustCompile("/sample/i")
)

const (
	pageSize = 25
)

type GetArgs struct {
	ID       string
	Page     uint
	PageSize uint
	PWD      string
	File     string
	Claims   *sv.Claims
}

func (s *Handler) bindGetArgs(c *gin.Context) (*GetArgs, error) {
	id := c.Param("resource_id")
	sha1 := sv.SHA1R.Find([]byte(id))
	if sha1 == nil {
		return nil, errors.Errorf("wrong resource provided resource_id=%v", id)
	}
	page := uint(1)
	if c.Query("page") != "" {
		p, err := strconv.Atoi(c.Query("page"))
		if err == nil && p > 1 {
			page = uint(p)
		}
	}
	return &GetArgs{
		ID:       id,
		Page:     page,
		PageSize: pageSize,
		PWD:      c.Query("pwd"),
		File:     c.Query("file"),
		Claims:   s.MakeClaims(c),
	}, nil
}

func (s *Handler) getList(ctx context.Context, args *GetArgs) (l *ra.ListResponse, err error) {
	limit := uint(args.PageSize)
	offset := (args.Page - 1) * args.PageSize
	l, err = s.api.ListResourceContent(ctx, args.Claims, args.ID, &sv.ListResourceContentArgs{
		Output: sv.OutputTree,
		Path:   args.PWD,
		Limit:  limit,
		Offset: offset,
	})
	return
}

type GetData struct {
	sw.ErrorData
	sw.CSRFData
	Args     *GetArgs
	Resource *ra.ResourceResponse
	List     *ra.ListResponse
	Item     *ra.ListItem
}

func (s *Handler) get(c *gin.Context) {
	var (
		d    GetData
		args *GetArgs
		res  *ra.ResourceResponse
		list *ra.ListResponse
		err  error
	)
	d.CSRF = csrf.GetToken(c)
	index := s.MakeTemplate(c, "index", &d)
	get := s.MakeTemplate(c, "resource/get", &d)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	args, err = s.bindGetArgs(c)
	d.Args = args
	if err != nil {
		d.Err = errors.Wrap(err, "wrong args provided")
		index.R(http.StatusBadRequest)
		return
	}
	res, err = s.api.GetResource(ctx, args.Claims, args.ID)
	if err != nil {
		d.Err = errors.Wrap(err, "failed to get resource")
		index.R(http.StatusInternalServerError)
		return
	}
	d.Resource = res
	if res == nil {
		d.Err = errors.Wrap(err, "resource not found")
		index.R(http.StatusNotFound)
		return
	}
	list, err = s.getList(ctx, args)
	if err != nil {
		d.Err = errors.Wrap(err, "failed to list resource")
		index.R(http.StatusInternalServerError)
		return
	}
	if len(list.Items) > 1 {
		d.List = list
	}
	d.Item, err = s.getBestItem(ctx, list, args)
	if err != nil {
		d.Err = errors.Wrap(err, "failed to get item")
		index.R(http.StatusInternalServerError)
		return
	}
	get.R(http.StatusOK)
}

func (s *Handler) getBestItem(ctx context.Context, l *ra.ListResponse, args *GetArgs) (i *ra.ListItem, err error) {
	if args.File != "" {
		for _, v := range l.Items {
			if v.PathStr == args.File {
				i = &v
				return
			}
		}
		l, err = s.api.ListResourceContent(ctx, args.Claims, args.ID, &sv.ListResourceContentArgs{
			Path: args.File,
		})
		if err != nil {
			return
		}
		if len(l.Items) > 0 {

			i = &l.Items[0]
			return
		}
	}
	if args.Page == 1 {
		for _, v := range l.Items {
			if v.MediaFormat == ra.Video && !sampleReg.MatchString(v.Name) {
				i = &v
				return
			}
		}
		for _, v := range l.Items {
			if v.MediaFormat == ra.Audio && !sampleReg.MatchString(v.Name) {
				i = &v
				return
			}
		}
		for _, v := range l.Items {
			if v.Type == ra.ListTypeFile {
				i = &v
				return
			}
		}
	}
	return
}
