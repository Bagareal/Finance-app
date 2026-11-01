package server

import (
	"net/http"

	"github.com/Bagareal/Finance-app/internal/store"
)

// proRoutes serves the Pro section: clients, categories and the payments split between them.
func (s *Server) proRoutes(p func(handler) http.HandlerFunc) {
	m := s.mux

	// clients and categories share the same shape; kind is "clients" or "categories"
	for _, kind := range []string{"clients", "categories"} {
		kind := kind
		base := "/api/pro/" + kind
		m.HandleFunc("GET "+base, p(func(w http.ResponseWriter, r *http.Request) (any, error) { return s.st.ListProNames(kind) }))
		m.HandleFunc("POST "+base, p(func(w http.ResponseWriter, r *http.Request) (any, error) {
			var n store.ProName
			if err := decode(r, &n); err != nil {
				return nil, err
			}
			if err := s.st.CreateProName(kind, &n); err != nil {
				return nil, err
			}
			return n, nil
		}))
		m.HandleFunc("PUT "+base+"/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
			id, err := pathID(r)
			if err != nil {
				return nil, err
			}
			var n store.ProName
			if err := decode(r, &n); err != nil {
				return nil, err
			}
			n.ID = id
			if err := s.st.UpdateProName(kind, &n); err != nil {
				return nil, err
			}
			return n, nil
		}))
		m.HandleFunc("DELETE "+base+"/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
			id, err := pathID(r)
			if err != nil {
				return nil, err
			}
			return nil, s.st.DeleteProName(kind, id)
		}))
	}

	m.HandleFunc("GET /api/pro/payments", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		q := r.URL.Query()
		return s.st.ListProPayments(q.Get("from"), q.Get("to"))
	}))
	m.HandleFunc("POST /api/pro/payments", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		var pay store.ProPayment
		if err := decode(r, &pay); err != nil {
			return nil, err
		}
		pay.ID = 0
		if err := s.st.CreateProPayment(&pay); err != nil {
			return nil, err
		}
		return s.st.GetProPayment(pay.ID)
	}))
	m.HandleFunc("PUT /api/pro/payments/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		var pay store.ProPayment
		if err := decode(r, &pay); err != nil {
			return nil, err
		}
		pay.ID = id
		if err := s.st.UpdateProPayment(&pay); err != nil {
			return nil, err
		}
		return s.st.GetProPayment(id)
	}))
	m.HandleFunc("DELETE /api/pro/payments/{id}", p(func(w http.ResponseWriter, r *http.Request) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		return nil, s.st.DeleteProPayment(id)
	}))
}
