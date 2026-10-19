package handler

import "github.com/gin-gonic/gin"

type InfoHandler struct{}

func NewInfoHandler() *InfoHandler {
	return &InfoHandler{}
}

func (h *InfoHandler) Get(c *gin.Context) {
	c.JSON(200, gin.H{
		"name":        "沈阳化工大学 ACM-ICPC 集训队",
		"englishName": "SYUCT ACM",
		"ojUrl":       "https://oj.syuctacm.cn",
		"githubOrg":   "https://github.com/syuctacm",
		"version":     "dev",
	})
}
