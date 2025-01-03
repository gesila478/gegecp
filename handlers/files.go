package handlers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"linux-panel/config"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type FileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	IsDir   bool      `json:"isDir"`
}

func ReadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不能为空"})
		return
	}

	// 读取文件内容
	content, err := ioutil.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回文件内容
	c.String(http.StatusOK, string(content))
}

func SaveFile(c *gin.Context) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不能为空"})
		return
	}

	// 确保目录存在
	dir := filepath.Dir(req.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败: " + err.Error()})
		return
	}

	// 写入文件内容
	if err := ioutil.WriteFile(req.Path, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文件保存成功"})
}

func ListFiles(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var fileInfos []FileInfo
	for _, f := range files {
		fileInfos = append(fileInfos, FileInfo{
			Name:    f.Name(),
			Size:    f.Size(),
			ModTime: f.ModTime(),
			IsDir:   f.IsDir(),
		})
	}

	c.JSON(http.StatusOK, fileInfos)
}

func UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	path := c.PostForm("path")
	filename := filepath.Join(path, file.Filename)

	// 确保目录存在
	if err := os.MkdirAll(path, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败: " + err.Error()})
		return
	}

	if err := c.SaveUploadedFile(file, filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文件上传成功"})
}

func DeleteFile(c *gin.Context) {
	path := c.Query("path")
	// log.Printf("请求删除文件: %s", path)

	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不能为空"})
		return
	}

	if err := os.Remove(path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "文件删除成功"})
}

func DownloadFile(c *gin.Context) {
	filename := c.Query("path")
	token := c.GetHeader("Authorization")
	// log.Printf("下载文件: 路径=%s, token=%s", filename, token)

	// 验证 token
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	// 验证 token 是否有效
	hasher := md5.New()
	hasher.Write([]byte(config.GlobalConfig.Auth.Username + config.GlobalConfig.Auth.Password))
	expectedToken := hex.EncodeToString(hasher.Sum(nil))

	if token != expectedToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 token"})
		return
	}

	// 清理和验证路径
	fullPath := filepath.Clean(filename)

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	// 设置下载头
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(fullPath)))
	c.Header("Content-Type", "application/octet-stream")
	c.File(fullPath)
}

// 处理文件列表请求
func HandleFilesList(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	files, err := os.ReadDir(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var fileList []gin.H
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			continue
		}

		// 获取文件权限
		fileMode := info.Mode()
		// 转换为字符串形式的权限表示 (例如: drwxr-xr-x)
		modeStr := fileMode.String()

		fileList = append(fileList, gin.H{
			"name":        file.Name(),
			"size":        info.Size(),
			"modTime":     info.ModTime(),
			"isDir":       file.IsDir(),
			"permissions": modeStr, // 添加权限信息
		})
	}

	c.JSON(http.StatusOK, fileList)
}

// 处理文件上传请求
func HandleFileUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	path := c.PostForm("path")
	if path == "" {
		path = "/"
	}

	dst := filepath.Join(path, file.Filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文件上传成功"})
}

// 处理文件下载请求
func HandleFileDownload(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不能为空"})
		return
	}

	c.File(path)
}

// 处理文件删除请求
func HandleFileDelete(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不能为空"})
		return
	}

	if err := os.Remove(path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文件删除成功"})
}

// 处理文件读取请求
func HandleFileRead(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不能为空"})
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.String(http.StatusOK, string(content))
}

// 处理文件保存请求
func HandleFileSave(c *gin.Context) {
	var req struct {
		Path    string `json:"path"`    // 文件路径
		Content string `json:"content"` // 文件内容
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件路径不能为空"})
		return
	}

	fmt.Printf("原始保存路径: %s\n", req.Path)

	// 处理路径
	savePath := req.Path
	// 如果路径以双斜杠开头，说明是从收藏夹打开的文件
	if strings.HasPrefix(savePath, "//") {
		// 获取文件名
		fileName := filepath.Base(savePath)

		// 从收藏列表中查找原始路径
		username := config.GlobalConfig.Auth.Username
		favorites, err := loadUserFavorites(username)
		if err == nil {
			for _, fav := range favorites {
				if filepath.Base(fav.Path) == fileName {
					savePath = fav.Path
					fmt.Printf("找到收藏文件的原始路径: %s\n", savePath)
					break
				}
			}
		}
	}

	fmt.Printf("最终保存路径: %s\n", savePath)

	// 确保目录存在
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("创建目录失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败: " + err.Error()})
		return
	}

	// 写入文件内容
	if err := os.WriteFile(savePath, []byte(req.Content), 0644); err != nil {
		fmt.Printf("写入文件失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败: " + err.Error()})
		return
	}

	fmt.Printf("文件保存成功: %s\n", savePath)
	c.JSON(http.StatusOK, gin.H{
		"message": "文件保存成功",
		"path":    savePath,
	})
}

// 处理文件权限修改请求
func HandleFileChmod(c *gin.Context) {
	var req struct {
		Path      string `json:"path"`
		Mode      string `json:"mode"`
		Recursive bool   `json:"recursive"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件路径不能为空"})
		return
	}

	// 将字符串转换为整数
	mode, err := strconv.ParseInt(req.Mode, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的权限值"})
		return
	}

	// 如果是递归模式且目标是目录
	if req.Recursive {
		err = filepath.Walk(req.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Chmod(path, os.FileMode(mode).Perm())
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "递归修改权限失败: " + err.Error()})
			return
		}
	} else {
		// 非递归模式，只修改当前文件/目录
		if err := os.Chmod(req.Path, os.FileMode(mode).Perm()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "修改权限失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "权限修改成功",
		"status":  "success",
	})
}
