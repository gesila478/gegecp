package handlers

import (
	"encoding/json"
	"gegecp/config"
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
	username := config.GlobalConfig.Auth.Username
	favorites, err := loadUserFavorites(username)
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

	username := config.GlobalConfig.Auth.Username
	if err := saveUserFavorites(username, favorites); err != nil {
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
		// 如果文件不存在，创建一个包含基本结构的文件
		initialData := UserFavorites{
			Username:  username,
			Favorites: []Favorite{},
		}
		data, err := json.MarshalIndent(initialData, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return nil, err
		}
		return initialData.Favorites, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// 如果文件存在但为空，初始化基本结构
	if len(data) == 0 {
		initialData := UserFavorites{
			Username:  username,
			Favorites: []Favorite{},
		}
		data, err := json.MarshalIndent(initialData, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return nil, err
		}
		return initialData.Favorites, nil
	}

	var userFavorites UserFavorites
	if err := json.Unmarshal(data, &userFavorites); err != nil {
		// 如果解析失败，尝试重新初始化文件
		initialData := UserFavorites{
			Username:  username,
			Favorites: []Favorite{},
		}
		data, err := json.MarshalIndent(initialData, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return nil, err
		}
		return initialData.Favorites, nil
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
