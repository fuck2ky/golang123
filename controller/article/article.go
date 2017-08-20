package article

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"gopkg.in/kataras/iris.v6"
	"github.com/microcosm-cc/bluemonday"
    "github.com/russross/blackfriday"
	"golang123/config"
	"golang123/model"
	"golang123/controller/common"
)

func queryList(isBackend bool, ctx *iris.Context) {
	SendErrJSON := common.SendErrJSON
	var articles []model.Article
	var categoryID int
	var pageNo int
	var err error

	if pageNo, err = strconv.Atoi(ctx.FormValue("pageNo")); err != nil {
		pageNo = 1
		err    = nil
	}
 
	if pageNo < 1 {
		pageNo = 1
	}

	offset   := (pageNo - 1) * config.ServerConfig.PageSize
	pageSize := config.ServerConfig.PageSize

	//默认按创建时间，降序来排序
	var orderField = "created_at"
	var orderASC   = "DESC"
	if ctx.FormValue("asc") == "1" {
		orderASC = "ASC"
	} else {
		orderASC = "DESC"	
	}

	cateIDStr := ctx.FormValue("cateId")
	if cateIDStr == "" {
		categoryID = 0	
	} else if categoryID, err = strconv.Atoi(cateIDStr); err != nil {
		fmt.Println(err.Error())
		SendErrJSON("分类ID不正确", ctx)
		return
	}

	if categoryID != 0 {
		var category model.Category
		if model.DB.First(&category, categoryID).Error != nil {
			SendErrJSON("分类ID不正确", ctx)
			return
		}
		var sql = `SELECT distinct(articles.id), articles.name, articles.browse_count, articles.status,  
					articles.created_at, articles.updated_at 
				FROM articles, article_category  
				WHERE articles.id = article_category.article_id  
				AND article_category.category_id = {categoryID} 
				ORDER BY {orderField} {orderASC}
				LIMIT {offset}, {pageSize}`
		sql = strings.Replace(sql, "{categoryID}", strconv.Itoa(categoryID), -1)
		sql = strings.Replace(sql, "{orderField}", orderField, -1)
		sql = strings.Replace(sql, "{orderASC}",   orderASC, -1)
		sql = strings.Replace(sql, "{offset}",     strconv.Itoa(offset), -1)
		sql = strings.Replace(sql, "{pageSize}",   strconv.Itoa(pageSize), -1)
		err = model.DB.Raw(sql).Scan(&articles).Error
		if err != nil {
			SendErrJSON("error", ctx)
			return
		}
		for i := 0; i < len(articles); i++ {
			articles[i].Categories = []model.Category{ category }
		}
	} else {
		orderStr := orderField + " " + orderASC
		if isBackend {
			err = model.DB.Offset(offset).Limit(pageSize).Order(orderStr).Find(&articles).Error
		} else {
			err = model.DB.Where("status = 1 OR status = 2").Offset(offset).Limit(pageSize).Order(orderStr).Find(&articles).Error
		}
		
		if err != nil {
			SendErrJSON("error", ctx)
			return
		}
		for i := 0; i < len(articles); i++ {
			if err = model.DB.Model(&articles[i]).Related(&articles[i].Categories, "categories").Error; err != nil {
				fmt.Println(err.Error())
				SendErrJSON("error", ctx)
				return
			}
		}
	}

	ctx.JSON(iris.StatusOK, iris.Map{
		"errNo" : model.ErrorCode.SUCCESS,
		"msg"   : "success",
		"data"  : iris.Map{
			"articles": articles,
		},
	})
}

// List 文章列表
func List(ctx *iris.Context) {
	queryList(false, ctx)
}

// AllList 文章列表，后台管理提供的接口
func AllList(ctx *iris.Context) {
	queryList(true, ctx)
}

// RecentList 用户最近文章
func RecentList(ctx *iris.Context) {
	SendErrJSON := common.SendErrJSON
	var userID int
	var userIDErr error
	if userID, userIDErr = ctx.ParamInt("userID"); userIDErr != nil {
		SendErrJSON("无效的id", ctx)
		return	
	}
	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		SendErrJSON("无效的id", ctx)
		return	
	}

	var articles []model.Article
	if err := model.DB.Where("user_id = ?", user.ID).Order("created_at DESC").Limit(5).Find(&articles).Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", ctx)
		return
	}
	ctx.JSON(iris.StatusOK, iris.Map{
		"errNo" : model.ErrorCode.SUCCESS,
		"msg"   : "success",
		"data"  : iris.Map{
			"articles": articles,
		},
	})	
}

// ListMaxComment 评论最多的文章，返回5条
func ListMaxComment(ctx *iris.Context) {
	SendErrJSON := common.SendErrJSON
	var articles []model.Article
	if err := model.DB.Order("comment_count DESC").Limit(5).Find(&articles).Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", ctx)
		return
	}
	ctx.JSON(iris.StatusOK, iris.Map{
		"errNo" : model.ErrorCode.SUCCESS,
		"msg"   : "success",
		"data"  : iris.Map{
			"articles": articles,
		},
	})
}

// ListMaxBrowse 访问量最多的文章，返回5条
func ListMaxBrowse(ctx *iris.Context) {
	SendErrJSON := common.SendErrJSON
	var articles []model.Article
	if err := model.DB.Order("browse_count DESC").Limit(5).Find(&articles).Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", ctx)
		return
	}
	ctx.JSON(iris.StatusOK, iris.Map{
		"errNo" : model.ErrorCode.SUCCESS,
		"msg"   : "success",
		"data"  : iris.Map{
			"articles": articles,
		},
	})
}

func save(isEdit bool, ctx *iris.Context) {
	SendErrJSON := common.SendErrJSON
	var article model.Article

	if err := ctx.ReadJSON(&article); err != nil {
		fmt.Println(err.Error())
		SendErrJSON("参数无效", ctx)
		return
	}

	var queryArticle model.Article
	if isEdit {
		if model.DB.First(&queryArticle, article.ID).Error != nil {
			SendErrJSON("无效的文章ID", ctx)
			return
		}
	}

	session        := ctx.Session();
	user           := session.Get("user").(model.User)
	article.UserID  = user.ID

	if isEdit {
		article.BrowseCount  = queryArticle.BrowseCount
		article.CreatedAt    = queryArticle.CreatedAt
		article.Status       = queryArticle.Status
		article.UpdatedAt    = time.Now()
	} else {
		article.BrowseCount  = 0
		article.Status       = model.ArticleVerifying
		user.Score           = user.Score + config.UserConfig.CreateArticleScore
		user.ArticleCount    = user.ArticleCount + 1
		session.Set("user", user)
	}

	article.Name    = strings.TrimSpace(article.Name)
	article.Content = strings.TrimSpace(article.Content)

	if (article.Name == "") {
		SendErrJSON("文章名称不能为空", ctx)
		return
	}
	
	if utf8.RuneCountInString(article.Name) > config.ServerConfig.MaxNameLen {
		msg := "文章名称不能超过" + strconv.Itoa(config.ServerConfig.MaxNameLen) + "个字符"
		SendErrJSON(msg, ctx)
		return
	}
	
	if article.Content == "" || utf8.RuneCountInString(article.Content) <= 0 {
		SendErrJSON("文章内容不能为空", ctx)
		return
	}
	
	if utf8.RuneCountInString(article.Content) > config.ServerConfig.MaxContentLen {	
		msg := "文章内容不能超过" + strconv.Itoa(config.ServerConfig.MaxContentLen) + "个字符"	
		SendErrJSON(msg, ctx)
		return
	}
	
	if article.Categories == nil || len(article.Categories) <= 0  {
		SendErrJSON("请选择版块", ctx)
		return
	}
	
	if len(article.Categories) > config.ServerConfig.MaxArticleCateCount {
		msg := "文章最多属于" + strconv.Itoa(config.ServerConfig.MaxArticleCateCount) + "个版块"
		SendErrJSON(msg, ctx)
		return
	}

	for i := 0; i < len(article.Categories); i++ {
		var category model.Category
		if err := model.DB.First(&category, article.Categories[i].ID).Error; err != nil {
			SendErrJSON("无效的版块id", ctx)
			return	
		}
		article.Categories[i] = category
	}

	var saveErr error;

	if isEdit {
		var sql = "DELETE FROM article_category WHERE article_id = ?"
		saveErr = model.DB.Exec(sql, article.ID).Error
		if saveErr == nil {
			saveErr = model.DB.Save(&article).Error	
		}
	} else {
		saveErr = model.DB.Create(&article).Error
		if saveErr == nil {
			// 发表文章后，用户的积分、文章数会增加，如果保存失败了，不作处理
			if userErr := model.DB.Save(&user).Error; userErr != nil {
				fmt.Println(userErr.Error())
			}
		}
	}

	if saveErr != nil {
		fmt.Println(saveErr.Error())
		SendErrJSON("error", ctx)
		return	
	}

	ctx.JSON(iris.StatusOK, iris.Map{
		"errNo" : model.ErrorCode.SUCCESS,
		"msg"   : "success",
		"data"  : article,
	})
}

// Create 创建文章
func Create(ctx *iris.Context) {
	save(false, ctx);	
}

// Update 更新文章
func Update(ctx *iris.Context) {
	save(true, ctx);	
}

// Info 获取文章信息
func Info(ctx *iris.Context) {
	SendErrJSON  := common.SendErrJSON
	reqStartTime := time.Now()
	var articleID int
	var paramsErr error

	if articleID, paramsErr = ctx.ParamInt("id"); paramsErr != nil {
		SendErrJSON("错误的文章id", ctx)
		return
	}

	var article model.Article

	if model.DB.First(&article, articleID).Error != nil {
		SendErrJSON("错误的文章id", ctx)
		return
	}

	article.BrowseCount++
	if err := model.DB.Save(&article).Error; err != nil {
		SendErrJSON("error", ctx)
		return
	}

	if err := model.DB.Model(&article).Related(&article.User, "users").Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", ctx)
		return
	}

	if err := model.DB.Model(&article).Related(&article.Categories, "categories").Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", ctx)
		return
	}

	if err := model.DB.Model(&article).Related(&article.Comments, "comments").Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", ctx)
		return
	}

	for i := 0; i < len(article.Comments); i++ {
		if err := model.DB.Model(&article.Comments[i]).Related(&article.Comments[i].User, "users").Error; err != nil {
			fmt.Println(err.Error())
			SendErrJSON("error", ctx)
			return
		}
		parentID := article.Comments[i].ParentID
		var parents []model.Comment
		for parentID != 0 {
			var parent model.Comment
			if err := model.DB.Where("parent_id = ?", parentID).Find(&parent).Error; err != nil {
				SendErrJSON("error", ctx)
				return
			}
			if err := model.DB.Model(&parent).Related(&parent.User, "users").Error; err != nil {
				fmt.Println(err.Error())
				SendErrJSON("error", ctx)
				return
			}
			parents = append(parents, parent)
			parentID = parent.ParentID
		}
		article.Comments[i].Parents = parents
	}

	if ctx.FormValue("f") != "md" {
		unsafe := blackfriday.MarkdownCommon([]byte(article.Content))
		article.Content = string(bluemonday.UGCPolicy().SanitizeBytes(unsafe))
	}

	fmt.Println("duration: ", time.Now().Sub(reqStartTime).Seconds())
	ctx.JSON(iris.StatusOK, iris.Map{
		"errNo" : model.ErrorCode.SUCCESS,
		"msg"   : "success",
		"data"  : iris.Map{
			"article": article,
		},
	})
}

// UpdateStatus 更新文章状态
func UpdateStatus(ctx *iris.Context) {
	SendErrJSON := common.SendErrJSON
	var reqData model.Article

	if err := ctx.ReadJSON(&reqData); err != nil {
		SendErrJSON("无效的id或status", ctx)
		return
	}

	articleID := reqData.ID
	status    := reqData.Status

	var article model.Article
	if err := model.DB.First(&article, articleID).Error; err != nil {
		SendErrJSON("无效的文章ID", ctx)
		return
	}
	
	if status != model.ArticleVerifying && status != model.ArticleVerifySuccess && status != model.ArticleVerifyFail {
		SendErrJSON("无效的文章状态", ctx)
		return
	}

	article.Status = status

	if err := model.DB.Save(&article).Error; err != nil {
		SendErrJSON("error", ctx)
		return
	}

	ctx.JSON(iris.StatusOK, iris.Map{
		"errNo" : model.ErrorCode.SUCCESS,
		"msg"   : "success",
		"data"  : iris.Map{
			"id"     : article.ID,
			"status" : article.Status,
		},
	})
}

