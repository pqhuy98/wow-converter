//go:build bake_debug

package reportshot

import (
 "context"
 "encoding/base64"
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "testing"
 "time"
)

func TestElementalVideos(t *testing.T) {
 ctx,cancel:=context.WithTimeout(context.Background(),2*time.Minute);defer cancel()
 b,err:=openBrowser(ctx);if err!=nil{t.Fatal(err)};defer b.close()
 if err=prepareViewport(ctx,b);err!=nil{t.Fatal(err)}
 for index,name:=range []string{"20261006-1805-33.1578412.mp4","20261006-1805-53.8815613.mp4"} {
  raw,err:=os.ReadFile(filepath.Join(`C:\Users\quang\AppData\Local\Packages\Microsoft.ScreenSketch_8wekyb3d8bbwe\TempState\Recordings`,name));if err!=nil{t.Fatal(err)}
  data,_:=json.Marshal("data:video/mp4;base64,"+base64.StdEncoding.EncodeToString(raw))
  if _,err=b.call(ctx,"Page.navigate",map[string]string{"url":"about:blank"});err!=nil{t.Fatal(err)}
  var duration float64
  script:=`new Promise((resolve,reject)=>{document.body.style.margin='0';document.body.style.background='#262626';let v=document.createElement('video');v.style.width='1440px';v.style.height='900px';v.style.objectFit='contain';document.body.append(v);window.__video=v;v.onloadedmetadata=()=>resolve(v.duration);v.onerror=()=>reject('video decode failed');v.src=`+string(data)+`;})`
  if err=b.evaluate(ctx,script,&duration);err!=nil{t.Fatal(err)}
  t.Log(name,duration)
  out:=fmt.Sprintf("D:/Projects/wow-converter/tmp/texture-animation-check/user-video-%d",index+1);if err=os.MkdirAll(out,0755);err!=nil{t.Fatal(err)}
  for frame,phase:=range []float64{.5,duration/2,max(.6,duration-1)} {
   script=fmt.Sprintf(`new Promise(r=>{let v=window.__video;v.onseeked=()=>requestAnimationFrame(()=>requestAnimationFrame(()=>r(true)));v.currentTime=%f;})`,phase)
   if err=b.evaluate(ctx,script,nil);err!=nil{t.Fatal(err)}
   reply,err:=b.call(ctx,"Page.captureScreenshot",map[string]string{"format":"png"});if err!=nil{t.Fatal(err)}
   var shot struct{Data string `json:"data"`};if err=json.Unmarshal(reply,&shot);err!=nil{t.Fatal(err)}
   png,err:=base64.StdEncoding.DecodeString(shot.Data);if err!=nil{t.Fatal(err)}
   if err=os.WriteFile(filepath.Join(out,fmt.Sprintf("frame-%d.png",frame)),png,0644);err!=nil{t.Fatal(err)}
  }
 }
}
