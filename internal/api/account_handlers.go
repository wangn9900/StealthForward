package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
)

// ListCloudAccountsHandler 列出所有云账号
func ListCloudAccountsHandler(c *gin.Context) {
	var accounts []models.CloudAccount
	database.DB.Find(&accounts)
	c.JSON(http.StatusOK, accounts)
}

// CreateCloudAccountHandler 创建云账号
func CreateCloudAccountHandler(c *gin.Context) {
	var account models.CloudAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Create(&account)
	c.JSON(http.StatusOK, account)
}

// UpdateCloudAccountHandler 更新云账号
func UpdateCloudAccountHandler(c *gin.Context) {
	id := c.Param("id")
	var account models.CloudAccount
	if err := database.DB.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
		return
	}
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Save(&account)
	c.JSON(http.StatusOK, account)
}

// DeleteCloudAccountHandler 删除云账号
func DeleteCloudAccountHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.CloudAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListSSHKeysHandler 列出 SSH 密钥
func ListSSHKeysHandler(c *gin.Context) {
	var keys []models.SSHKey
	database.DB.Find(&keys)
	c.JSON(http.StatusOK, keys)
}

// CreateSSHKeyHandler 创建 SSH 密钥
func CreateSSHKeyHandler(c *gin.Context) {
	var key models.SSHKey
	if err := c.ShouldBindJSON(&key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Create(&key)
	c.JSON(http.StatusOK, key)
}

// UpdateSSHKeyHandler 更新 SSH 密钥
func UpdateSSHKeyHandler(c *gin.Context) {
	id := c.Param("id")
	var key models.SSHKey
	if err := database.DB.First(&key, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
		return
	}
	if err := c.ShouldBindJSON(&key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Save(&key)
	c.JSON(http.StatusOK, key)
}

// DeleteSSHKeyHandler 删除 SSH 密钥
func DeleteSSHKeyHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.SSHKey{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
