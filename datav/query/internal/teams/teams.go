// Copyright 2023 xObserve.io Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package teams

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xObserve/xObserve/query/internal/admin"
	"github.com/xObserve/xObserve/query/internal/user"
	"github.com/xObserve/xObserve/query/pkg/colorlog"
	"github.com/xObserve/xObserve/query/pkg/common"
	"github.com/xObserve/xObserve/query/pkg/db"
	"github.com/xObserve/xObserve/query/pkg/e"
	"github.com/xObserve/xObserve/query/pkg/models"
)

var logger = colorlog.RootLogger.New("logger", "teams")

func GetTeams(c *gin.Context) {
	teamId, _ := strconv.ParseInt(c.Query("teamId"), 10, 64)

	u := user.CurrentUser(c)
	var tenantId int64
	var err error
	if teamId == 0 {
		if u != nil {
			tenantId = u.CurrentTenant
		} else {
			tenantId = models.DefaultTenantId
		}
	} else {
		tenantId, err = models.QueryTenantIdByTeamId(c.Request.Context(), teamId)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(400, common.RespError(e.TeamNotExist))
				return
			}
			logger.Warn("get tenant id by team id error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}
	}

	teams, err := GetTeamsByTenantId(c.Request.Context(), tenantId, u)
	if err != nil {
		logger.Warn("get teams error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	c.JSON(200, common.RespSuccess(teams))
}

func GetTeamsByTenantId(ctx context.Context, tenantId int64, u *models.User) (models.Teams, error) {
	teams := make(models.Teams, 0)
	q := fmt.Sprintf(`SELECT id,name,brief,is_public,created_by FROM team WHERE tenant_id='%d'`, tenantId)

	if u == nil {
		q = fmt.Sprintf("%s AND is_public=true", q)
	} else {
		// user can see the teams he is in
		if !u.Role.IsAdmin() {
			members, err := models.QueryVisibleTeamsByUserId(ctx, tenantId, u.Id)
			if err != nil {
				return nil, err
			}

			if len(members) == 0 {
				return teams, nil
			}

			if len(members) == 1 {
				q = fmt.Sprintf("%s AND id = '%d'", q, members[0])
			} else {
				for i, m := range members {
					if i == 0 {
						q = fmt.Sprintf("%s AND id in ('%d'", q, m)
						continue
					}

					if i == len(members)-1 {
						q = fmt.Sprintf("%s,'%d')", q, m)
						continue
					}

					q = fmt.Sprintf("%s,'%d'", q, m)
				}
			}
		}
	}

	rows, err := db.Conn.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		team := &models.Team{}
		err := rows.Scan(&team.Id, &team.Name, &team.Brief, &team.IsPublic, &team.CreatedById)
		if err != nil {
			logger.Warn("get all users scan error", "error", err)
			continue
		}

		user, _ := models.QueryUserById(ctx, team.CreatedById)
		team.CreatedBy = user.Username

		count := 0
		err = db.Conn.QueryRowContext(ctx, "SELECT count(*) FROM team_member WHERE team_id=?", team.Id).Scan(&count)
		if err != nil {
			logger.Warn("select team member count error", "error", err)
		}

		team.MemberCount = count
		if u != nil {
			member, _ := models.QueryTeamMember(ctx, team.Id, u.Id)
			if member != nil && member.Id != 0 {
				team.CurrentUserRole = member.Role
			}
		} else {
			team.CurrentUserRole = models.ROLE_VIEWER
		}

		teams = append(teams, team)
	}

	sort.Sort(teams)

	return teams, nil
}
func GetTeam(c *gin.Context) {
	id, _ := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if id == 0 {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	team, err := models.QueryTeam(c.Request.Context(), id, "")
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Warn("get team  error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}

		c.JSON(200, common.RespSuccess(models.Team{}))
		return
	}

	c.JSON(200, common.RespSuccess(team))
}

func GetTeamByDashId(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	teamId, err := models.QueryDashboardBelongsTo(c.Request.Context(), id)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Warn("get team  error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}

		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	team, err := models.QueryTeam(c.Request.Context(), teamId, "")
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Warn("get team  error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}

		c.JSON(200, common.RespSuccess(models.Team{}))
		return
	}

	c.JSON(200, common.RespSuccess(team))
}

func GetTeamMembers(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	members := make(models.TeamMembers, 0)
	rows, err := db.Conn.QueryContext(c.Request.Context(), "SELECT user_id,role,created FROM team_member WHERE team_id=?", id)
	if err != nil && err != sql.ErrNoRows {
		logger.Warn("get team members error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}
	defer rows.Close()
	for rows.Next() {
		member := &models.TeamMember{}
		err := rows.Scan(&member.Id, &member.Role, &member.Created)
		if err != nil {
			logger.Warn("get team members scan error", "error", err)
			continue
		}

		u, _ := models.QueryUserById(c.Request.Context(), member.Id)
		member.Username = u.Username
		member.RoleSortWeight = models.RoleSortWeight(member.Role)
		member.TeamId = id
		members = append(members, member)
	}

	sort.Sort(members)

	c.JSON(200, common.RespSuccess(members))
}

func GetTeamMember(c *gin.Context) {
	teamId, _ := strconv.ParseInt(c.Param("teamId"), 10, 64)
	userId, _ := strconv.ParseInt(c.Param("userId"), 10, 64)

	if teamId == 0 || userId == 0 {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	member, err := models.QueryTeamMember(c.Request.Context(), teamId, userId)
	if err != nil {
		logger.Warn("get team member error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	c.JSON(200, common.RespSuccess(member))
}

type AddMemberReq struct {
	TeamId  int64           `json:"teamId"`
	Members []string        `json:"members"`
	Role    models.RoleType `json:"role"`
}

func AddTeamMembers(c *gin.Context) {
	req := &AddMemberReq{}
	c.Bind(&req)

	members := req.Members
	role := req.Role

	if req.TeamId == 0 || len(members) == 0 || !role.IsValid() {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	u := user.CurrentUser(c)
	isTeamAdmin, err := models.IsTeamAdmin(c.Request.Context(), req.TeamId, u.Id)
	if err != nil {
		logger.Warn("check team admin error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	if u.CurrentTenant == 0 {
		c.JSON(400, common.RespError("You must select a tenant first"))
		return
	}

	// only global admin and team admin can do this
	if !u.Role.IsAdmin() && !isTeamAdmin {
		c.JSON(403, common.RespError(e.NoPermission))
		return
	}

	team, err := models.QueryTeam(c.Request.Context(), req.TeamId, "")
	if err != nil {
		logger.Warn("get team error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	memberIds := make([]int64, 0)
	// check whether user is in current tenant
	for _, member := range members {
		memberId, err := models.QueryUserIdByName(member)
		if err != nil {
			logger.Warn("Get user id by name error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}
		inTenant, err := models.IsUserInTenant(memberId, team.TenantId)
		if err != nil {
			logger.Warn("check user in tenant error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}
		if !inTenant {
			c.JSON(400, common.RespError(fmt.Sprintf("user %s is not in current tenant", member)))
			return
		}

		memberIds = append(memberIds, memberId)
	}

	now := time.Now()
	for _, memberId := range memberIds {
		_, err := db.Conn.ExecContext(c.Request.Context(), "INSERT INTO team_member (tenant_id,team_id,user_id,role,created,updated) VALUES (?,?,?,?,?,?)", u.CurrentTenant, req.TeamId, memberId, role, now, now)
		if err != nil {
			logger.Warn("add team member error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}
	}

	c.JSON(200, common.RespSuccess(nil))
}

func DeleteTeamMember(c *gin.Context) {
	teamId, _ := strconv.ParseInt(c.Param("teamId"), 10, 64)
	memberId, _ := strconv.ParseInt(c.Param("memberId"), 10, 64)

	if teamId == 0 || memberId == 0 {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	u := user.CurrentUser(c)
	if memberId == u.Id {
		c.JSON(400, common.RespError("cannot delete yourself"))
		return
	}

	// only team admin can do this
	if !u.Role.IsAdmin() {
		isTeamAdmin, err := models.IsTeamAdmin(c.Request.Context(), teamId, u.Id)
		if err != nil {
			logger.Warn("check team admin error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}

		if !isTeamAdmin {
			c.JSON(403, common.RespError(e.NoPermission))
			return
		}
	}

	_, err := db.Conn.ExecContext(c.Request.Context(), "DELETE FROM team_member where team_id=? and user_id=?", teamId, memberId)
	if err != nil {
		logger.Warn("delete team member error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	c.JSON(200, common.RespSuccess(nil))
}

func UpdateTeam(c *gin.Context) {
	team := &models.Team{}
	c.Bind(&team)

	if team.Id == 0 || team.Name == "" {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	u := user.CurrentUser(c)
	if !u.Role.IsAdmin() {
		isTeamAdmin, err := models.IsTeamAdmin(c.Request.Context(), team.Id, u.Id)
		if err != nil {
			logger.Warn("check team admin error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}

		if !isTeamAdmin {
			c.JSON(403, common.RespError(e.NoPermission))
			return
		}
	}

	_, err := db.Conn.ExecContext(c.Request.Context(), "UPDATE team SET name=?, is_public=? WHERE id=?", team.Name, team.IsPublic, team.Id)
	if err != nil {
		logger.Warn("update team error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	c.JSON(200, common.RespSuccess(nil))
}

func UpdateTeamMember(c *gin.Context) {
	member := &models.TeamMember{}
	c.Bind(&member)

	if member.TeamId == 0 || member.Id == 0 || !member.Role.IsValid() {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	team, err := models.QueryTeam(c.Request.Context(), member.TeamId, "")
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(400, common.RespError(e.TeamNotExist))
			return
		}
		c.JSON(500, common.RespInternalError())
		return
	}

	u := user.CurrentUser(c)
	if member.Id == u.Id {
		c.JSON(400, common.RespError("cannot change your own role"))
		return
	}

	if !u.Role.IsAdmin() {
		isTeamAdmin, err := models.IsTeamAdmin(c.Request.Context(), team.Id, u.Id)
		if err != nil {
			logger.Warn("check team admin error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}

		if !isTeamAdmin {
			c.JSON(403, common.RespError(e.NoPermission))
			return
		}
	}
	_, err = db.Conn.ExecContext(c.Request.Context(), "UPDATE team_member SET role=?,updated=? WHERE team_id=? and user_id=?", member.Role, time.Now(), member.TeamId, member.Id)
	if err != nil {
		logger.Warn("update team member error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	c.JSON(200, common.RespSuccess(nil))
}

func DeleteTeam(c *gin.Context) {
	teamId, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	if teamId == 0 {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	u := user.CurrentUser(c)

	if !models.IsSuperAdmin(u.Id) {
		c.JSON(403, common.RespError("Only super admin can do this"))
		return
	}

	t, err := models.QueryTeam(c.Request.Context(), teamId, "")
	if err != nil {
		logger.Warn("query team error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	// check tenant has more than one team
	var teamCount int
	err = db.Conn.QueryRow("SELECT count(*) FROM team WHERE tenant_id=?", t.TenantId).Scan(&teamCount)
	if err != nil {
		logger.Warn("query team count error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	if teamCount <= 1 {
		c.JSON(400, common.RespError("tenant must have at least one team"))
		return
	}

	tx, err := db.Conn.Begin()
	if err != nil {
		logger.Warn("start sql transaction error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(c.Request.Context(), "DELETE FROM team WHERE id=?", teamId)
	if err != nil {
		logger.Warn("delete team  error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	_, err = tx.ExecContext(c.Request.Context(), "DELETE FROM team_member WHERE team_id=?", teamId)
	if err != nil {
		logger.Warn("delete team member error", "error", err)
		c.JSON(500, common.RespInternalError())
	}

	_, err = tx.ExecContext(c.Request.Context(), "DELETE FROM sidemenu WHERE team_id=?", teamId)
	if err != nil {
		logger.Warn("delete team sidemenu error", "error", err)
		c.JSON(500, common.RespInternalError())
	}

	err = tx.Commit()
	if err != nil {
		logger.Warn("commit sql transaction error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	admin.WriteAuditLog(c.Request.Context(), u.Id, admin.AuditDeleteTeam, strconv.FormatInt(teamId, 10), t)

	c.JSON(200, common.RespSuccess(nil))
}

func LeaveTeam(c *gin.Context) {
	teamId, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if teamId == 0 {
		c.JSON(400, common.RespError(e.ParamInvalid))
		return
	}

	u := user.CurrentUser(c)
	isTeamAdmin, err := models.IsTeamAdmin(c.Request.Context(), teamId, u.Id)
	if err != nil {
		logger.Warn("check team admin error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	// if user is team admin, can't leave team, he must degrade to normal member first
	if isTeamAdmin {
		c.JSON(400, common.RespError("team admin can't leave team, please degrade to normal member first"))
		return
	}

	userId := user.CurrentUserId(c)
	_, err = db.Conn.ExecContext(c.Request.Context(), "DELETE FROM team_member where team_id=? and user_id=?", teamId, userId)
	if err != nil {
		logger.Warn("leave team  error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	c.JSON(200, common.RespSuccess(nil))
}

func SwitchTeam(c *gin.Context) {
	userId := user.CurrentUserId(c)
	teamId := c.Param("teamId")

	err := SetTeamForUser(c.Request.Context(), teamId, userId)
	if err != nil {
		logger.Warn("update side menu error", "error", err)
		c.JSON(500, common.RespInternalError())
		return
	}

	c.JSON(200, common.RespSuccess(nil))
}

func SetTeamForUser(ctx context.Context, teamId string, userId int64) error {
	_, err := db.Conn.ExecContext(ctx, "UPDATE user SET current_team=? WHERE id=?", teamId, userId)
	if err != nil {
		return err
	}

	return nil
}

func GetTeamsForUser(c *gin.Context) {
	u := user.CurrentUser(c)
	teams := make([]*models.Team, 0)
	if u != nil {
		teamIds, err := models.QueryVisibleTeamsByUserId(c.Request.Context(), u.CurrentTenant, u.Id)
		if err != nil {
			logger.Warn("query teams for user error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}

		for _, id := range teamIds {
			team, err := models.QueryTeam(c.Request.Context(), id, "")
			if err != nil {
				logger.Warn("query team error", "error", err)
				continue
			}

			teams = append(teams, team)
		}
	} else {
		_, teamId, err := models.GetUserTenantAndTeamId(c.Request.Context(), nil)
		if err != nil {
			logger.Warn("query teams for user error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}

		team, err := models.QueryTeam(c.Request.Context(), teamId, "")
		if err != nil {
			logger.Warn("query team error", "error", err)
			c.JSON(500, common.RespInternalError())
			return
		}

		teams = append(teams, team)
	}

	c.JSON(200, common.RespSuccess(teams))
}