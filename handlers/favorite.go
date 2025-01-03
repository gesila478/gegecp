package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type Favorite struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

type UserFavorites struct {
	Username  string     `json:"username"`
	Favorites []Favorite `json:"favorites"`
}

// 获取收藏列表
func GetFavorites(c *gin.Context) {
	// 从session中获取用户名
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	favorites, err := loadUserFavorites(username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载收藏列表失败"})
		return
	}

	c.JSON(http.StatusOK, favorites)
}

// 更新收藏列表
func UpdateFavorites(c *gin.Context) {
	var favorites []Favorite
	if err := c.ShouldBindJSON(&favorites); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 从session中获取用户名
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if err := saveUserFavorites(username.(string), favorites); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存收藏列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "收藏列表更新成功",
		"status":  "success",
	})
}

// 从文件加载用户的收藏列表
func loadUserFavorites(username string) ([]Favorite, error) {
	favoritesDir := "data/favorites"
	if err := os.MkdirAll(favoritesDir, 0755); err != nil {
		return nil, err
	}

	filePath := filepath.Join(favoritesDir, username+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []Favorite{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var userFavorites UserFavorites
	if err := json.Unmarshal(data, &userFavorites); err != nil {
		return nil, err
	}

	return userFavorites.Favorites, nil
}

// 保存用户的收藏列表到文件
func saveUserFavorites(username string, favorites []Favorite) error {
	favoritesDir := "data/favorites"
	if err := os.MkdirAll(favoritesDir, 0755); err != nil {
		return err
	}

	userFavorites := UserFavorites{
		Username:  username,
		Favorites: favorites,
	}

	data, err := json.MarshalIndent(userFavorites, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(favoritesDir, username+".json")
	return os.WriteFile(filePath, data, 0644)
}
