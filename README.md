# Golang Swagger API Document
> Based on [mikunalpha/goas](https://github.com/mikunalpha/goas) repository.

Generate [OpenAPI Specification](https://swagger.io/specification) json file with comments in Go.

## 基本註解寫法

```
將以下資訊寫在此API服務的main.go檔案(這樣才可以抓的到所有import的package) 
'＊'代表整行註解都是必填，沒有此符號的話，代表都不用填。
```

### Info
```
// @ServersURL http://10.200.252.121:8000
// @Version 1.0.0 
// @Title Backend API
// @Description API usually works as expected. But sometimes its not true.
// @ContactName Abcd
// @ContactEmail abce@email.com
// @ContactURL http://someurl.oxox
// @TermsOfServiceUrl http://someurl.oxox
// @LicenseURL https://en.wikipedia.org/wiki/MIT_License
// @LicenseName MIT
```

**@ServersURL** - ＊ Host      
**@Version** - ＊ 版本   
**@Title** - ＊ 標題   
**@Description** - 說明   
**@ContactName** - 聯絡人   
**@ContactEmail** - 聯絡信箱   
**@ContactURL** - 聯絡人網站   
**@TermsOfServiceUrl** - 服務條款網址   
**@LicenseURL** - License種類   
**@LicenseName** - License的網址   
　　
　　
### Operation
將以下資訊寫在此專案下的任一檔案(通常是寫在該api的function上)，但**務必**寫在該function正上方，中間請勿有任何**段落** 

```
// @Title Get user list of a group.
// @Description Get users related to a specific group.
// @Param  group_id  path  int  true  "Id of a specific group."
// @ParamStruct  reporting.DailyReportingConds
// @Success  200  {object}  UsersResponse  "UsersResponse JSON"
// @Failure  400  {object}  ErrorResponse  "ErrorResponse JSON"
// @Resource users
// @Router /api/group/{group_id}/users [get]
```

**@Title** - 此api的標題
**@Description** - 此api的描述   
**@Param** - 此api的各個param   
**@ParamStruct** - 直接import整個struct為parameters(可以與上面@Param一起使用)，任何required、description參數都寫在該struct裡面  
**@Success/@Failure** - ＊ 此api的回傳結果。使用Success或Failure在產出結果上沒有差異，主要是根據後面的http status code(200, 400, 500 ...)來指定不一定的回傳結果   
**@Resource** - tags的意思，可以幫不同API歸類群組(沒填預設歸類在"default"群組)   
**@Router** - ＊ 手動寫下api路徑，以及其method   

   
## Struct 範例
```perl
type DailyReportingConds struct {
    StartTime    core.DateTime `json:"starttime,required" description:"Start Time"`
    EndTime      core.DateTime `json:"endtime,required" description:"End Time"`
    ShareLogin   core.String   `json:"sharelogin" description:"股東"`
    GeneralAgent core.String   `json:"generalagent" description:"總代"`
    Agent        core.String   `json:"agent" description:"代理"`
    DataSource   core.Int      `json:"datasource" description:"資料來源 SQL:0,Analysis:1,ES:2"`
}
```

   
## 使用方法

按照上面的教學，在你的專案寫下註解之後，遵循以下作法就可以產出對應API Doc   

打開cmd/bash(*~~vscode的不行，原因不明~~*)
```
go get -u -v --insecure github.com/Hao1995/go-swagger/cmd/goas
```
進入你要產生API Doc的專案位置
```
cd /d “C:\gotool\src\gitlab.paradise-soft.com.tw\routing\apis\mock”
```
輸入以下指令，以產出API Doc(以reporting, qa分支為例)
```
%GOPATH%\bin\goas --output reporting-qa.json
```
如果你有設定環境變數GOBIN=C:\gotool\bin的話，你可以輸入以下指令
```
%GOBIN%\goas --output reporting-qa.json
```
接著檢查專案位置，就可以看到reporting-qa.json的產出了

如果發現goas.exe不存在%GOBIN%裡面，手動編譯執行檔，並回去執行產文件的指令
```
go install github.com/Hao1995/go-swagger/cmd/goas
```

打開此reporting-qa.json，並複製其內容
貼到[Swagger Editor](http://editor.swagger.io/)
就可以輸入參數並測試API了

如果上述產API Doc時發生問題  
檢查"C:\gotool\bin"下面有無"goas.exe"的檔案  
沒有的話請執行以下，以編譯執行檔
```
go install github.com/Hao1995/go-swagger/cmd/goas
```
或是直接到repo下，執行go build產生執行檔，並移動檔案到"C:\gotool\bin"下面
```
cd /d "C:\gotool\src\gitlab.paradise-soft.com.tw\backend\goas\cmd\goas"
go build
<!-- 移動goas.exe到"C:\gotool\bin"下面 -->
```
輸入以下指令，以產出API Doc
```
%GOPATH%\bin\goas --output reporting-qa.json
```
   
## 其他
### OpenAPI Specification
有關API的資料型態、parameter的參數，都可以上官方網站去查詢。 >> [OpenAPI Specification](https://swagger.io/specification)

### 檔案命名規則
主要以qa、prod為主  
[repo name]-[branch].json  
EX:  
  reporting-qa.json  
  athena-prod.json  
  dwh-prod.json  

### Interface解法
因為寫@Success、@Failure的時候，是直接讀取struct裡面的fields，但目前許多報表都是採用interface的作法
```perl
type Hits struct{
	Items interface{}
	Pager paging.Pager
}
```
導致此程式無法產出對應的回傳資料(因為不知道interface指的是誰)

而為了讓大家的API Doc是可以測試，並且讓閱讀者也了解回傳的資料，所以在寫API Doc的時候，需要多寫一個type，讓API的產出順利。以下以每日報表為例：
```perl
type DailyReportingPager struct {
    Pager *paging.Pager
    Items []*DailyReportingItem
}
```
```
// @Success 200 {object} reporting.DailyReportingPager "每日報表回傳格式"
```

### API無回傳值的解法
將struct的位置，改為'empty'，即可回傳空白的API response
```
...
// @Success  200  {object}  empty  "UsersResponse JSON"
...
```

</br>

# Swagger-UI
server位置在lv-dev(10.200.252.121:1180)
```
打開xshell，並拖曳reporting-qa.json檔案到"/swagger"資料夾
```
```
打開http://10.200.252.121:1280/
並在搜索列貼上http://10.200.252.121:1180/reporting-qa.json
就可以看到API Doc內容了
可以嘗試"try it out"來測試API
```
## 其他問題
如果發現swagger-ui http://10.200.252.121:1280/ 或是swagger-file-server http://10.200.252.121:1180/ 沒有回應的話
```
docker ps
```
確認swagger-ui或swagger-file-server是否存活
如果是exit的狀態
```
docker restart swagger-ui
docker restart swagger-file-server
```
如果依然不行，請交由Ted, Harvey, Edward處理