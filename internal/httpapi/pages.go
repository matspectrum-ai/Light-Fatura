package httpapi

import (
	"net/http"
	"path"
)

func (s *FullServer) authPage(w http.ResponseWriter,r *http.Request){http.ServeFile(w,r,path.Join("Light","auth.html"))}
func (s *FullServer) forgotPage(w http.ResponseWriter,r *http.Request){http.ServeFile(w,r,path.Join("Light","auth.html"))}
func (s *FullServer) resetPage(w http.ResponseWriter,r *http.Request){http.ServeFile(w,r,path.Join("Light","auth.html"))}
func (s *FullServer) adminPage(w http.ResponseWriter,r *http.Request){http.ServeFile(w,r,path.Join("Light","admin.html"))}
