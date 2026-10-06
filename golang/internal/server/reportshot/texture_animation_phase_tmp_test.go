//go:build bake_debug

package reportshot

import (
 "context"
 "encoding/json"
 "os"
 "path/filepath"
 "testing"
 "time"
)

func TestTextureAnimationPhases(t *testing.T) {
 ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute); defer cancel()
 b, err := openBrowser(ctx); if err != nil { t.Fatal(err) }; defer b.close()
 if err = prepareViewport(ctx,b); err != nil { t.Fatal(err) }
 _ = b.setCacheDisabled(ctx,true)
 model, out := os.Getenv("ANIM_CAPTURE_MODEL"), os.Getenv("ANIM_CAPTURE_OUT")
 if _,_,err = captureConverter(ctx,b,Request{BaseURL:"http://127.0.0.1:3001",Model:model,Sequence:"Stand 1"},[]int{0}); err != nil { t.Fatal(err) }
 var found bool
 if err = b.evaluate(ctx,`(()=>{let c=document.querySelector('canvas'),f=c?.[Object.keys(c).find(k=>k.startsWith('__reactFiber'))];while(f){let h=f.memoizedState;for(let n=0;h&&n<100;n++,h=h.next){let v=h.memoizedState?.current;if(v?.model&&typeof v.updateAnimations==='function'){window.__animInstance=v;return true;}}f=f.return;}return false;})()`, &found); err != nil || !found { t.Fatalf("instance %v/%t",err,found) }
 if err = os.MkdirAll(out,0755);err != nil { t.Fatal(err) }
 ref:=&cameraReference{Target:[3]float64{0,0,85},Eye:[3]float64{250,0,85},Up:[3]float64{0,0,1},Height:180,ModelMatrix:[16]float64{1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1}}
 for _,phase := range []string{"0","1100"} {
  var state any
  if err = b.evaluate(ctx,`(()=>{let i=window.__animInstance;i.counter=`+phase+`;i.forced=true;i.updateAnimations(0);return {counter:i.counter,sequence:i.sequence,uv:i.uvAnims.map(x=>Array.from(x)),textures:Array.from(i.layerTextures)};})()`, &state); err != nil { t.Fatal(err) }
  data,_:=json.MarshalIndent(state,"","  "); if err=os.WriteFile(filepath.Join(out,"state-"+phase+".json"),data,0644);err!=nil {t.Fatal(err)}
  raw,err:=shootConverterView(ctx,b,"front",ref);if err!=nil {t.Fatal(err)}
  if err=os.WriteFile(filepath.Join(out,"phase-"+phase+".png"),raw,0644);err!=nil {t.Fatal(err)}
 }
}
