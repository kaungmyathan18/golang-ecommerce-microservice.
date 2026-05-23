package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type Gateway struct {
	product   *httputil.ReverseProxy
	order     *httputil.ReverseProxy
	inventory *httputil.ReverseProxy
	payment   *httputil.ReverseProxy
	auth      *httputil.ReverseProxy
}

func New(productURL, orderURL, inventoryURL, paymentURL, authURL string) (*Gateway, error) {
	product, err := newProxy(productURL)
	if err != nil {
		return nil, err
	}
	order, err := newProxy(orderURL)
	if err != nil {
		return nil, err
	}
	inventory, err := newProxy(inventoryURL)
	if err != nil {
		return nil, err
	}
	payment, err := newProxy(paymentURL)
	if err != nil {
		return nil, err
	}
	auth, err := newProxy(authURL)
	if err != nil {
		return nil, err
	}
	return &Gateway{product: product, order: order, inventory: inventory, payment: payment, auth: auth}, nil
}

func newProxy(raw string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	p := httputil.NewSingleHostReverseProxy(u)
	p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, `{"error":"upstream unavailable"}`, http.StatusBadGateway)
	}
	return p, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case strings.HasPrefix(path, "/api/v1/auth"):
		g.auth.ServeHTTP(w, r)
	case strings.HasPrefix(path, "/api/v1/products"), strings.HasPrefix(path, "/api/v1/categories"):
		g.product.ServeHTTP(w, r)
	case strings.HasPrefix(path, "/api/v1/orders"):
		g.order.ServeHTTP(w, r)
	case strings.HasPrefix(path, "/api/v1/inventory"):
		g.inventory.ServeHTTP(w, r)
	case strings.HasPrefix(path, "/api/v1/payments"):
		g.payment.ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}
