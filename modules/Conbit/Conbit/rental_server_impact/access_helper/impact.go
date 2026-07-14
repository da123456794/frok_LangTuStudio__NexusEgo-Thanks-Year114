package access_helper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/Conbit/bundle"
	"github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/challenges"
	"github.com/LangTuStudio/Conbit/i18n"
	"github.com/LangTuStudio/Conbit/nodes/defines"
)

type AuthenticatorWithToken interface {
	Authenticator
	GetToken() string
}

func ImpactServer(ctx context.Context, node defines.Node, options *Options) (omegaCore Conbit.MicroOmega, err error) {
	defer func() {
		if err != nil {
			err = i18n.FuzzyTransErr(err)
		}
	}()
	NormalizeImpactOptionServerTarget(options.ImpactOption)
	targetNames := ServerTargetLogNames(options.ImpactOption.ServerCode)
	infoLine(i18n.T(i18n.S_executing_login_sequence))
	if options.MaximumWaitTime > 0 {
		ctx, _ = context.WithTimeout(ctx, options.MaximumWaitTime)
	}
	infoLine(i18n.T(i18n.S_updating_tag_in_omega_net))
	if node.CheckNetTag("access-point") {
		return nil, fmt.Errorf("%s", i18n.T(i18n.S_net_position_conflict_only_one_access_point_can_exist))
	}
	node.SetTags("access-point")
	// make auth client and wrap authenticator & challenge solver
	var authenticator Authenticator
	var challengeSolver challenges.CanSolveChallenge
	authenticator, challengeSolver, err = makeAuthenticatorAndChallengeSolver(options.ImpactOption, options.WriteBackToken)
	if err != nil {
		return nil, err
	}

	// connect to minecraft solver
	var unReadyOmega Conbit.UnReadyMicroOmega
	{
		mcServerConnectCtx := ctx
		if options.ServerConnectionTimeout != 0 {
			mcServerConnectCtx, _ = context.WithTimeout(ctx, options.ServerConnectionTimeout)
		}
		password := "No"
		if options.ImpactOption.ServerPassword != "" {
			password = "Yes"
		}
		displayCode := serverTargetValueForLog(options.ImpactOption.ServerCode)
		infof(i18n.T(i18n.S_connecting_to_mc_server), targetNames.neteaseKind, targetNames.codeLabel, displayCode, password)
		conn, err := loginMCServerWithRetry(
			mcServerConnectCtx,
			authenticator,
			options.ImpactOption.ServerCode,
			options.ImpactOption.ServerPassword,
			options.ServerConnectRetryTimes,
		)
		if err != nil {
			return nil, err
		}
		unReadyOmega = bundle.NewAccessPointMicroOmega(node, conn, IsDomainGameTarget(options.ImpactOption.ServerCode))
		s := sha256.Sum256([]byte(conn.IdentityData().NeteaseSid))
		c := sha256.Sum256([]byte(options.ImpactOption.ServerCode))
		for i := range s {
			s[i] ^= c[i]
		}
		node.SetValue("HashedServerCode", defines.FromString(hex.EncodeToString(s[:])))
	}
	// unReadyOmega, err = makeNodeOmegaCoreFromConn(node, conn)
	omegaCore = unReadyOmega

	// POST PROCESSES
	challengeSolvingCtx := ctx
	if options.ChallengeSolvingTimeout != 0 {
		challengeSolvingCtx, _ = context.WithTimeout(ctx, options.ChallengeSolvingTimeout)
	}
	if !IsOnlineGameTarget(options.ImpactOption.ServerCode) {
		if err := copeWithRentalServerChallenge(challengeSolvingCtx, omegaCore, challengeSolver); err != nil {
			return nil, err
		}
	}
	if options.ReasonWithPrivilegeStuff {
		err := reasonWithPrivilegeStuff(ctx, omegaCore, options.PrivilegeStuffOptions, targetNames)
		if err != nil {
			return nil, err
		}
	}
	if options.MakeBotCreative {
		unReadyOmega.PostponeActionsAfterChallengePassed("make bot creative", func() {
			makeBotCreative(omegaCore.GetGameControl())
		})
	}
	if options.DisableCommandBlock {
		unReadyOmega.PostponeActionsAfterChallengePassed("disable command block", func() {
			disableCommandBlock(omegaCore.GetGameControl())
		})
	}
	unReadyOmega.NotifyChallengePassed()
	go waitDead(omegaCore)
	// wait everything stable
	// we need to wait until some packets received before using some api
	time.Sleep(time.Second * 3)
	return omegaCore, nil
}
