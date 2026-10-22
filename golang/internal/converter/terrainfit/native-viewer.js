'use strict';
function decodeTerrainBLP(raw,level=0){
 let v=new DataView(raw),b=new Uint8Array(raw);if(raw.byteLength<1180||v.getUint32(0,true)!==0x31504c42||v.getUint32(4,true)!==1)throw Error('Unsupported benchmark BLP');if(!Number.isInteger(level)||level<0||level>15)throw Error('Invalid BLP mip');let bits=v.getUint32(8,true),width=Math.max(1,v.getUint32(12,true)>>level),height=Math.max(1,v.getUint32(16,true)>>level),offset=v.getUint32(28+level*4,true),count=width*height;if(![0,1,4,8].includes(bits)||offset+count+Math.ceil(count*bits/8)>b.length)throw Error('Invalid benchmark BLP');let rgba=new Uint8Array(count*4);for(let i=0;i<count;i++){let c=156+b[offset+i]*4,a=255;if(bits===8)a=b[offset+count+i];else if(bits===4)a=((b[offset+count+(i>>1)]>>((i%2)*4))&15)*17;else if(bits===1)a=((b[offset+count+(i>>3)]>>(i%8))&1)*255;rgba[i*4]=b[c+2];rgba[i*4+1]=b[c+1];rgba[i*4+2]=b[c];rgba[i*4+3]=a}return {rgba,width,height};
}
// Live native terrain rendering: per-corner materials, mask priority,16 base
// variations and individually mipmapped subtiles decoded from the actual BLPs.
function createNativeTerrainRenderer(gl,resources){
 const compile=(type,source)=>{let s=gl.createShader(type);gl.shaderSource(s,source);gl.compileShader(s);if(!gl.getShaderParameter(s,gl.COMPILE_STATUS))throw Error(gl.getShaderInfoLog(s));return s};
 const p=gl.createProgram();gl.attachShader(p,compile(gl.VERTEX_SHADER,`#version 300 es
 precision highp float;precision highp int;precision highp usampler2D;
 uniform mat4 matrix;uniform vec2 offset;uniform ivec2 tileOffset;uniform int cells;
 uniform float heightOrigin;uniform bool flatView;uniform usampler2D labels;uniform usampler2D variants;uniform sampler2D heights;uniform usampler2D patches;
 out vec2 uv;out vec2 sourceUV;out vec3 world;flat out uvec4 textureIndices;
 const vec2 vertices[6]=vec2[6](vec2(0,0),vec2(1,0),vec2(0,1),vec2(0,1),vec2(1,0),vec2(1,1));
 void main(){ivec2 cell=ivec2(gl_InstanceID%cells,gl_InstanceID/cells);ivec2 globalCell=cell+tileOffset;uv=vertices[gl_VertexID];sourceUV=(vec2(cell)+uv)/float(cells);ivec2 corner=globalCell+ivec2(uv);float z=texelFetch(heights,corner,0).r;world=vec3(sourceUV.x+offset.x,-sourceUV.y-offset.y,flatView?0.:(z-heightOrigin)/533.3333333);gl_Position=matrix*vec4(world,1.);
 if(texelFetch(patches,globalCell,0).r!=0u){gl_Position=vec4(2,2,2,1);}
 uvec4 c=uvec4(texelFetch(labels,globalCell+ivec2(0,1),0).r,texelFetch(labels,globalCell+ivec2(1,1),0).r,texelFetch(labels,globalCell+ivec2(1,0),0).r,texelFetch(labels,globalCell,0).r);
 uint ids[4]=uint[4](c.x,c.y,c.z,c.w);for(int i=1;i<4;i++){for(int j=i;j>0;j--){if(ids[j]<ids[j-1]){uint t=ids[j];ids[j]=ids[j-1];ids[j-1]=t;}}}
 textureIndices=uvec4(65535u);textureIndices[0]=ids[0]*32u+16u+texelFetch(variants,globalCell,0).r;int layer=1;for(int i=1;i<4;i++){if(ids[i]==ids[i-1])continue;uint id=ids[i],mask=0u;if(c.y==id)mask|=1u;if(c.x==id)mask|=2u;if(c.z==id)mask|=4u;if(c.w==id)mask|=8u;textureIndices[layer++]=id*32u+mask;}}
 `));
 gl.attachShader(p,compile(gl.FRAGMENT_SHADER,`#version 300 es
 precision highp float;precision highp int;precision highp sampler2DArray;
 in vec2 uv;in vec2 sourceUV;in vec3 world;flat in uvec4 textureIndices;
 uniform sampler2DArray ground0;uniform sampler2DArray ground1;uniform sampler2DArray ground2;uniform sampler2DArray ground3;uniform sampler2DArray ground4;uniform sampler2DArray ground5;uniform sampler2DArray ground6;uniform sampler2DArray ground7;
 uniform sampler2D errorImage;uniform bool errorView;uniform bool lighting;uniform bool sequential;out vec4 color;
 vec4 sampleGround(uint id){if(id==65535u)return vec4(0);vec3 q=vec3(uv,float(id%256u));uint group=id/256u;if(group==0u)return texture(ground0,q);if(group==1u)return texture(ground1,q);if(group==2u)return texture(ground2,q);if(group==3u)return texture(ground3,q);if(group==4u)return texture(ground4,q);if(group==5u)return texture(ground5,q);if(group==6u)return texture(ground6,q);return texture(ground7,q);}
 void main(){vec4 c;if(sequential){c=sampleGround(textureIndices.r);for(int k=1;k<4;k++){if(textureIndices[k]==65535u)continue;vec4 overlay=sampleGround(textureIndices[k]);c=mix(c,overlay,overlay.a);}}else{c=sampleGround(textureIndices.a);c=mix(sampleGround(textureIndices.b),c,c.a);c=mix(sampleGround(textureIndices.g),c,c.a);c=mix(sampleGround(textureIndices.r),c,c.a);}if(errorView)c=texture(errorImage,sourceUV);if(lighting){vec3 n=normalize(cross(dFdx(world),dFdy(world)));c.rgb*=.42+.58*abs(dot(n,normalize(vec3(.3,-.4,1.))));}color=vec4(c.rgb,1.);}
 `));
 gl.linkProgram(p);if(!gl.getProgramParameter(p,gl.LINK_STATUS))throw Error(gl.getProgramInfoLog(p));
 let locations={};for(const name of ['matrix','offset','tileOffset','cells','heightOrigin','flatView','labels','variants','heights','patches','lighting','sequential','errorImage','errorView',...Array.from({length:8},(_,i)=>'ground'+i)])locations[name]=gl.getUniformLocation(p,name);
 const vao=gl.createVertexArray();let state=null;
 function tex2(width,height,internal,format,type,data,owner=resources){let t=gl.createTexture();gl.activeTexture(gl.TEXTURE0);gl.bindTexture(gl.TEXTURE_2D,t);gl.pixelStorei(gl.UNPACK_ALIGNMENT,1);gl.texImage2D(gl.TEXTURE_2D,0,internal,width,height,0,format,type,data);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MIN_FILTER,gl.NEAREST);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MAG_FILTER,gl.NEAREST);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_S,gl.CLAMP_TO_EDGE);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_T,gl.CLAMP_TO_EDGE);owner.push(()=>gl.deleteTexture(t));return t}
 const bytes=s=>Uint8Array.from(atob(s),c=>c.charCodeAt(0));
 async function configure(assignment,mesh,patches,base,palette,cells,owner=resources,signal){
  let w=assignment.Width,h=assignment.Height,labels=bytes(assignment.Labels),variants=bytes(assignment.BaseVariations),mask=new Uint8Array(w*h);
  for(const tile of patches||[]){let x=tile.x*cells,y=tile.y*cells;for(let j=0;j<cells;j++)for(let i=0;i<cells;i++)mask[(y+j)*w+x+i]=tile.cells[j*cells+i]?1:0}
  let height=new Float32Array(mesh.positions.length);for(let i=0;i<height.length;i++)height[i]=mesh.positions[i][2];
  let data={w,h,cells,sequential:assignment.Compositing==='sequential',labelBytes:labels,mask,labels:tex2(w+1,h+1,gl.R8UI,gl.RED_INTEGER,gl.UNSIGNED_BYTE,labels,owner),variants:tex2(w,h,gl.R8UI,gl.RED_INTEGER,gl.UNSIGNED_BYTE,variants,owner),heights:tex2(w+1,h+1,gl.R32F,gl.RED,gl.FLOAT,height,owner),patches:tex2(w,h,gl.R8UI,gl.RED_INTEGER,gl.UNSIGNED_BYTE,mask,owner),arrays:[]};
  for(let group=0;group<Math.ceil(palette/8);group++){let t=gl.createTexture();gl.bindTexture(gl.TEXTURE_2D_ARRAY,t);gl.texStorage3D(gl.TEXTURE_2D_ARRAY,7,gl.RGBA8,64,64,256);gl.texParameteri(gl.TEXTURE_2D_ARRAY,gl.TEXTURE_MIN_FILTER,gl.LINEAR_MIPMAP_LINEAR);gl.texParameteri(gl.TEXTURE_2D_ARRAY,gl.TEXTURE_MAG_FILTER,gl.LINEAR);gl.texParameteri(gl.TEXTURE_2D_ARRAY,gl.TEXTURE_WRAP_S,gl.CLAMP_TO_EDGE);gl.texParameteri(gl.TEXTURE_2D_ARRAY,gl.TEXTURE_WRAP_T,gl.CLAMP_TO_EDGE);data.arrays.push(t);owner.push(()=>gl.deleteTexture(t))}
  for(let family=0;family<palette;family++){
   let r=await fetch(base+'/tileset/tile-'+String(family).padStart(2,'0')+'.blp',{signal});if(!r.ok)throw Error('Missing native tileset '+family);let raw=await r.arrayBuffer();
   gl.activeTexture(gl.TEXTURE0);gl.bindTexture(gl.TEXTURE_2D_ARRAY,data.arrays[Math.floor(family/8)]);
   for(let level=0;level<7;level++){
    let {rgba,width,height}=decodeTerrainBLP(raw,level),side=64>>level;if(width!==(512>>level)||height!==(256>>level))throw Error('Invalid native atlas layout');
    let tile=new Uint8Array(side*side*4);for(let layer=0;layer<32;layer++){let x=(layer%4+(layer>=16?4:0))*side,y=Math.floor(layer%16/4)*side;for(let row=0;row<side;row++)tile.set(rgba.subarray(((y+row)*width+x)*4,((y+row)*width+x+side)*4),row*side*4);gl.texSubImage3D(gl.TEXTURE_2D_ARRAY,level,0,0,(family%8)*32+layer,side,side,1,gl.RGBA,gl.UNSIGNED_BYTE,tile)}
   }
  }
  return data;
 }
 function draw(matrix,x,y,lighting,errorTexture,errorView,heightOrigin,flat=false){if(!state)return;let s=state;gl.useProgram(p);gl.bindVertexArray(vao);gl.uniformMatrix4fv(locations.matrix,false,matrix);gl.uniform2f(locations.offset,x,y);gl.uniform2i(locations.tileOffset,x*s.cells,y*s.cells);gl.uniform1i(locations.cells,s.cells);gl.uniform1f(locations.heightOrigin,heightOrigin);gl.uniform1i(locations.flatView,flat?1:0);gl.uniform1i(locations.lighting,lighting?1:0);gl.uniform1i(locations.sequential,s.sequential?1:0);gl.uniform1i(locations.errorView,errorView?1:0);for(let i=0;i<8;i++){gl.activeTexture(gl.TEXTURE0+i);gl.bindTexture(gl.TEXTURE_2D_ARRAY,s.arrays[i]||s.arrays[0]);gl.uniform1i(locations['ground'+i],i)}gl.activeTexture(gl.TEXTURE8);gl.bindTexture(gl.TEXTURE_2D,errorTexture);gl.uniform1i(locations.errorImage,8);for(const [name,unit]of [['labels',9],['variants',10],['heights',11],['patches',12]]){gl.activeTexture(gl.TEXTURE0+unit);gl.bindTexture(gl.TEXTURE_2D,s[name]);gl.uniform1i(locations[name],unit)}gl.drawArraysInstanced(gl.TRIANGLES,0,6,s.cells*s.cells);gl.activeTexture(gl.TEXTURE0);}
 async function texture2D(url,level=0,owner=resources,signal){let response=await fetch(url,{signal});if(!response.ok)throw Error('Missing patch atlas '+url);let {rgba,width,height}=decodeTerrainBLP(await response.arrayBuffer(),level);gl.activeTexture(gl.TEXTURE0);let t=tex2(width,height,gl.RGBA8,gl.RGBA,gl.UNSIGNED_BYTE,rgba,owner);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MIN_FILTER,gl.LINEAR_MIPMAP_LINEAR);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MAG_FILTER,gl.LINEAR);gl.generateMipmap(gl.TEXTURE_2D);return t;}
 return {configure,draw,texture2D,use(data){state=data},reset(){state=null},get ready(){return !!state},validationBlock(tx,ty){if(!state)return null;let s=state,best=null,bestScore=-1;for(let y=1;y<s.cells-5;y+=4)for(let x=1;x<s.cells-5;x+=4){let ok=true,ids=new Set();for(let j=0;j<4;j++)for(let i=0;i<4;i++){if(s.mask[(ty*s.cells+y+j)*s.w+tx*s.cells+x+i])ok=false;let at=(ty*s.cells+y+j)*(s.w+1)+tx*s.cells+x+i;ids.add(s.labelBytes[at]);ids.add(s.labelBytes[at+1]);ids.add(s.labelBytes[at+s.w+1]);ids.add(s.labelBytes[at+s.w+2])}if(ok&&ids.size>bestScore){best={x,y};bestScore=ids.size}}return best}};
}
