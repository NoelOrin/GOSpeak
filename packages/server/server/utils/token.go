package utils

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"os"
	"time"
)

type TokenClient struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func (t *TokenClient) SetRefreshToken(username string) {
	SetClaims := TokenClient{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // 有效时间
			IssuedAt:  jwt.NewNumericDate(time.Now()),                     // 签发时间
			NotBefore: jwt.NewNumericDate(time.Now()),                     // 生效时间
			// Issuer:    os.Getenv("JWT_ISSUER"),                            // 签发人
			ID: "1", // JWT ID用于标识该JWT
			// Audience:  []string{"somebody_else"},                          // 用户
		},
	}

	// 使用指定的加密方式和声明类型创建新令牌
	tokenStruct := jwt.NewWithClaims(jwt.SigningMethodHS256, SetClaims)
	// 获得完整的、签名的令牌
	token, err := tokenStruct.SignedString([]byte(os.Getenv("JWT_KEY")))
	if err != nil {

	}
	fmt.Println(token)
}

func (t *TokenClient) SetAccessToken() {

}

func (t *TokenClient) RefreshToken() {

}

func (t *TokenClient) VerifyToken(token string) {
	//解析、验证并返回token。
	tokenObj, err := jwt.ParseWithClaims(token, &TokenClient{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_KEY")), nil
	})

	if err != nil {

	}

	if claims, ok := tokenObj.Claims.(*TokenClient); ok && tokenObj.Valid {
		fmt.Printf("%v %v\n", claims.Username, claims.RegisteredClaims)
	}
}

func (t *TokenClient) ExtractInfoFromToken() {

}
