package info_collect_utils

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/LangTuStudio/Conbit/Conbit/fbauth"
	"github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/access_helper"
	"github.com/LangTuStudio/Conbit/i18n"
	"github.com/LangTuStudio/Conbit/utils/input"

	"github.com/LangTuStudio/Conbit/internal/termlog"
	"golang.org/x/term"
)

func LoadTokenPath() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(fmt.Errorf(i18n.T(i18n.S_fail_to_get_user_home_dir), homedir))
		homedir = "."
	}
	fbconfigdir := filepath.Join(homedir, ".config", "fastbuilder")
	os.MkdirAll(fbconfigdir, 0700)
	token := filepath.Join(fbconfigdir, "fbtoken")
	return token
}

func ReadToken() (string, error) {
	content, err := os.ReadFile(LoadTokenPath())
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func DeleteToken() error {
	return os.Remove(LoadTokenPath())
}

func GetUserInput(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	return strings.TrimSpace(input), err
}

func GetUserPasswordInput(prompt string) (string, error) {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Printf("\n")
	if err != nil {
		return strings.TrimSpace(string(bytePassword)), err
	}
	password := strings.TrimSpace(string(bytePassword))
	if password != "" {
		return password, nil
	} else {
		return GetUserPasswordInput(prompt)
	}
}

func GetRentalServerCode() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(i18n.T(i18n.S_please_enter_rental_server_code))
	code, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	code = strings.TrimSpace(strings.TrimRight(code, "\r\n"))
	if code == "" {
		return GetRentalServerCode()
	}
	fmt.Print(i18n.T(i18n.S_please_enter_rental_server_password))
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Printf("\n")
	password := strings.TrimSpace(string(bytePassword))
	if err != nil {
		return code, password, err
	}
	return code, password, nil
}

func WriteFBToken(token string, tokenPath string) {
	if fp, err := os.Create(tokenPath); err != nil {
		fmt.Printf(i18n.T(i18n.S_fail_to_create_token_file), err)
	} else {
		_, err = fp.WriteString(token)
		if err != nil {
			fmt.Printf(i18n.T(i18n.S_fail_to_write_token), err)
		}
		fp.Close()
	}
}

const (
	AUTH_SERVER_FB_OFFICIAL = "https://user.fastbuilder.pro"
	AUTH_SERVER_LILIYA      = "https://api.liliya233.uk"
	AUTH_SERVER_CUSTOM      = "__custom__"
)

var AUTH_SERVER_NAMES = map[string]string{
	AUTH_SERVER_FB_OFFICIAL: i18n.T(i18n.S_auth_server_name_official),
	AUTH_SERVER_LILIYA:      i18n.T(i18n.S_auth_server_name_liliya),
}
var AUTH_SERVER_SELECT_STRINGS = []string{
	"1. " + i18n.T(i18n.S_auth_server_name_official),
	"2. " + i18n.T(i18n.S_auth_server_name_liliya),
	"3. custom auth server",
}

func TranslateInputToAuthServer(input string) (authServer string, authServerName string, err error) {
	input = strings.TrimSpace(input)
	if input == "1" {
		return AUTH_SERVER_FB_OFFICIAL, i18n.T(i18n.S_auth_server_name_official), nil
	} else if input == "2" {
		return AUTH_SERVER_LILIYA, i18n.T(i18n.S_auth_server_name_liliya), nil
	} else if input == "3" {
		return AUTH_SERVER_CUSTOM, "custom auth server", nil
	}
	if normalizedAuthServer, err := fbauth.NormalizeAuthServerURL(input); err == nil {
		return normalizedAuthServer, TranslateAuthServerToAuthServerName(normalizedAuthServer), nil
	}
	return "", "", fmt.Errorf("invalid input, please input 1 ~ 3 or a valid auth server URL")
}

func TranslateAuthServerToAuthServerName(authServer string) (authServerName string) {
	if authServerName, ok := AUTH_SERVER_NAMES[authServer]; ok {
		return authServerName
	} else {
		return fmt.Sprintf(i18n.T(i18n.S_auth_server_name_user_specific), authServer)
	}
}

func supportsUsernamePasswordLogin(authServer string) bool {
	return authServer == AUTH_SERVER_FB_OFFICIAL || authServer == AUTH_SERVER_LILIYA
}

func looksLikeUserTokenCandidate(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "\"{") {
		return true
	}
	if strings.HasPrefix(value, "cookie:") || strings.HasPrefix(strings.ToLower(value), "cookie ") {
		return true
	}
	return strings.Contains(value, "/")
}

func ReadUserInfoAndUpdateImpactOptions(impactOptions *access_helper.ImpactOption) (err error) {
	impactOptions.UserName, impactOptions.UserPassword, impactOptions.UserToken,
		impactOptions.ServerCode, impactOptions.ServerPassword,
		impactOptions.AuthServer,
		err =
		ReadUserInfo(
			impactOptions.UserName, impactOptions.UserPassword, impactOptions.UserToken,
			impactOptions.ServerCode, impactOptions.ServerPassword,
			impactOptions.AuthServer,
		)
	// if impactOptions.AuthServer == AUTH_SERVER_FB_OFFICIAL {
	// 	panic(i18n.T(i18n.S_auth_server_version_not_support))
	// }
	return err
}

func ReadUserInfo(userName, userPassword, userToken, serverCode, serverPassword, authServer string) (string, string, string, string, string, string, error) {
	var err error
	authServerName := ""

	flagAuthServerGiven := authServer != ""
	flagUserTokenGiven := userToken != ""
	flagUserNameGiven := userName != ""
	tokenFileToken := ""
	{
		fileUserToken, err := ReadToken()
		if err == nil && fileUserToken != "" {
			tokenFileToken = fileUserToken
		}
	}
	flagHasFileToken := tokenFileToken != ""
	flagNeedInteractivelyInput := false

	if flagUserTokenGiven {
		if !flagAuthServerGiven {
			if strings.HasPrefix(userToken, "w9/") {
				authServer = AUTH_SERVER_FB_OFFICIAL
				flagAuthServerGiven = true
			} else if strings.HasPrefix(userToken, "y8/") {
				authServer = AUTH_SERVER_LILIYA
				flagAuthServerGiven = true
			} else {
				fmt.Println(i18n.T(i18n.S_please_input_auth_server_address_or_specific_auth_server))
				authServer = input.GetValidInput()
			}
		}
	} else if flagUserNameGiven {
		if flagAuthServerGiven && !supportsUsernamePasswordLogin(authServer) {
			if looksLikeUserTokenCandidate(userName) {
				userToken = userName
				userName = ""
			} else {
				return userName, userPassword, userToken, serverCode, serverPassword, authServer, fmt.Errorf("自定义验证服务器启动时仅支持直接提供 login_token / cookie，请使用 --user-token 或在交互中直接粘贴 token")
			}
		}
		if !flagAuthServerGiven {
			flagAuthServerGiven = true
			authServer = AUTH_SERVER_FB_OFFICIAL
		}
		if userToken == "" {
			authServerName = TranslateAuthServerToAuthServerName(authServer)
			for userPassword == "" {
				userPassword, err = GetUserPasswordInput(fmt.Sprintf(i18n.T(i18n.S_please_input_auth_server_user_password), authServerName))
				if err != nil {
					break
				}
			}
		}
	} else if flagHasFileToken {
		fmt.Print(i18n.T(i18n.S_login_with_current_token))
		if !input.GetInputYN(true) {
			DeleteToken()
			flagNeedInteractivelyInput = true
		} else {
			userToken = tokenFileToken
			if !flagAuthServerGiven {
				if strings.HasPrefix(userToken, "w9/") {
					authServer = AUTH_SERVER_FB_OFFICIAL
					flagAuthServerGiven = true
				} else if strings.HasPrefix(userToken, "y8/") {
					authServer = AUTH_SERVER_LILIYA
					flagAuthServerGiven = true
				}
			}
			if !flagAuthServerGiven {
				fmt.Println(i18n.T(i18n.S_please_input_auth_server_address_or_specific_auth_server))
				authServer = input.GetValidInput()
			}
		}
	} else {
		flagNeedInteractivelyInput = true
	}

	for flagNeedInteractivelyInput {
		authServerName = TranslateAuthServerToAuthServerName(authServer)
		for authServer == "" {
			fmt.Printf(i18n.T(i18n.S_please_select_auth_server), strings.Join(AUTH_SERVER_SELECT_STRINGS, "\n"))
			selection := input.GetValidInput()
			authServer, authServerName, err = TranslateInputToAuthServer(selection)
			if err != nil {
				termlog.Errorf("%s", err.Error())
				continue
			}
			if authServer == AUTH_SERVER_CUSTOM {
				fmt.Println(i18n.T(i18n.S_please_input_auth_server_address_or_specific_auth_server))
				customAuthServer := input.GetValidInput()
				authServer, err = fbauth.NormalizeAuthServerURL(customAuthServer)
				if err != nil {
					authServer = ""
					authServerName = ""
					termlog.Errorf("%s", err.Error())
					continue
				}
				authServerName = TranslateAuthServerToAuthServerName(authServer)
			}
		}
		if supportsUsernamePasswordLogin(authServer) {
			for userName == "" {
				userName, _ = GetUserInput(fmt.Sprintf(i18n.T(i18n.S_please_input_auth_server_user_name_or_token), authServerName))
				if looksLikeUserTokenCandidate(userName) {
					userToken = userName
					userName = ""
					break
				}
			}
			if userToken == "" {
				for userPassword == "" {
					userPassword, err = GetUserPasswordInput(fmt.Sprintf(i18n.T(i18n.S_please_input_auth_server_user_password), authServerName))
					if err == nil {
						break
					}
				}
				fbClient, err := fbauth.CreateClient(&fbauth.ClientOptions{
					AuthServer: authServer,
				})
				if err != nil {
					termlog.Errorf(i18n.T(i18n.S_cannot_connect_to_auth_server), authServer, err)
					time.Sleep(3 * time.Second)
					continue
				}
				userPasswordSHA256 := fmt.Sprintf("%x", sha256.Sum256([]byte(userPassword)))
				authResp, err := fbClient.Auth(context.Background(), "::DRY::", "::DRY::", "", "", userName, userPasswordSHA256)
				token, _ := authResp["token"].(string)
				if err != nil || token == "" {
					termlog.Errorf("%s %v", i18n.T(i18n.S_invalid_auth_server_user_account), err)
					userName = ""
					userPassword = ""
					continue
				}
				userToken = token
			}
		} else {
			for userToken == "" {
				userToken, _ = GetUserInput(fmt.Sprintf("请输入验证服务器[%v]的 login_token / cookie (留空则使用服务端全局登录态): ", authServerName))
				if strings.TrimSpace(userToken) == "" {
					break
				}
				if !looksLikeUserTokenCandidate(userToken) {
					termlog.Errorf("自定义验证服务器请直接输入 login_token / cookie，或留空使用服务端全局登录态")
					userToken = ""
				}
			}
		}
		break
	}

	for serverCode == "" {
		serverCode, serverPassword, err = GetRentalServerCode()
		if err != nil {
			return userName, userPassword, userToken, serverCode, serverPassword, authServer, err
		}
	}
	return userName, userPassword, userToken, serverCode, serverPassword, authServer, nil
}
