package web

import (
	"fmt"
	"net/http"

	"path"

	"github.com/gorilla/mux"
)

type routerOptions struct {
	path   string
	hosts  []string
	isRoot bool
}

type RouterHandler struct {
	methods      []string
	path         string
	routeHandler RouteHandler
}

type Router struct {
	options    routerOptions
	handlers   []RouterHandler
	middleware MiddlewareStack
}

func NewRouter(path string, opts ...RouterOption) *Router {
	options := newRouterOptions(path, opts...)
	router := Router{
		handlers: make([]RouterHandler, 0),
		options:  options,
		middleware: MiddlewareStack{
			stack: make([]MiddlewareFunc, 0),
			skips: nil,
		},
	}
	return &router
}

type RouterOption func(*routerOptions)

func Hosts(hosts ...string) RouterOption {
	return func(o *routerOptions) {
		o.hosts = hosts
	}
}

func newRouterOptions(path string, opts ...RouterOption) routerOptions {
	opt := routerOptions{
		path: path,
	}
	for _, o := range opts {
		o(&opt)
	}
	return opt
}

func (app *App) Router(path string, opts ...RouterOption) *Router {
	router := NewRouter(path, opts...)
	app.routers = append(app.routers, router)
	return router
}

func (router *Router) HandleMethod(methods []string, path string, routeHandler RouteHandler) *Router {
	gh := RouterHandler{
		methods:      methods,
		path:         path,
		routeHandler: routeHandler,
	}
	router.handlers = append(router.handlers, gh)
	return router
}

func (router *Router) Get(path string, routeHandler RouteHandler) *Router {
	return router.HandleMethod([]string{"GET"}, path, routeHandler)
}

func (router *Router) Post(path string, routeHandler RouteHandler) *Router {
	return router.HandleMethod([]string{"POST"}, path, routeHandler)
}

func (router *Router) Put(path string, routeHandler RouteHandler) *Router {
	return router.HandleMethod([]string{"PUT"}, path, routeHandler)
}

func (router *Router) Delete(path string, routeHandler RouteHandler) *Router {
	return router.HandleMethod([]string{"DELETE"}, path, routeHandler)
}

func (router *Router) Patch(path string, routeHandler RouteHandler) *Router {
	return router.HandleMethod([]string{"PATCH"}, path, routeHandler)
}

func (router *Router) Any(path string, routeHandler RouteHandler) *Router {
	return router.HandleMethod([]string{"GET", "POST", "PUT", "DELETE", "PATCH"}, path, routeHandler)
}

func (router *Router) Use(mw ...MiddlewareFunc) {
	router.middleware.Use(mw...)
}

func (app *App) registerRouterHandleFunc(router *Router, h RouterHandler, host *string) {
	var muxRouter *mux.Router
	if host == nil {
		muxRouter = app.muxRouter
	} else {
		muxRouter = app.muxRouter.Host(*host).Subrouter()
	}
	routerPath := router.options.path
	muxRouter.HandleFunc(path.Join(routerPath, h.path), func(res http.ResponseWriter, req *http.Request) {
		ctx := NewDefaultContext()
		ctx.logger = app.opts.logger
		ctx.broker = app.opts.broker
		ctx.res = res
		ctx.req = req
		ctx.cookies = &Cookies{
			res: res,
			req: req,
		}
		ctx.session = app.getSession(res, req)
		ctx.tmplEngine = app.tmplEngine
		// Handle Controller Panic
		defer func() {
			if r := recover(); r != nil {
				var e error
				switch t := r.(type) {
				case error:
					e = t
				case string:
					e = fmt.Errorf(t)
				default:
					e = fmt.Errorf(fmt.Sprint(t))
				}
				app.HandleError(res, req, e)
			}
		}()
		h := h.routeHandler
		// Apply Root Middlewares
		if router != app.rootRouter {
			h = app.rootRouter.middleware.handle(h)
		}
		// Apply Router (Scoped) Middlewares
		h = router.middleware.handle(h)
		err := h(ctx)
		// Handle Controller Error
		if err != nil {
			app.HandleError(res, req, err)
		}
	}).Methods(h.methods...)
}

func (app *App) initRouterHandlers(router *Router) {
	for _, h := range router.handlers {
		if len(router.options.hosts) == 0 {
			app.registerRouterHandleFunc(router, h, nil)
		} else {
			for _, host := range router.options.hosts {
				app.registerRouterHandleFunc(router, h, &host)
			}
		}
	}
}
