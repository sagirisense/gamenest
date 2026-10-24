import type {Directive} from 'vue'

const svgNS='http://www.w3.org/2000/svg'
let nextID=0
const cleanup=new WeakMap<HTMLElement,()=>void>()

// A small, static displacement map bends only the rim. Text is on a separate
// layer, and maps are regenerated on size changes, never on pointer/scroll frames.
export const vGlass:Directive<HTMLElement>={
 mounted(el){
  el.classList.add('glass-surface')
  if(!/Chrome|Chromium|Edg\//.test(navigator.userAgent))return
  const svg=document.createElementNS(svgNS,'svg')
  svg.setAttribute('aria-hidden','true')
  svg.setAttribute('width','0');svg.setAttribute('height','0')
  svg.style.cssText='position:fixed;pointer-events:none'
  const filter=document.createElementNS(svgNS,'filter')
  const id=`glass-rim-${++nextID}`
  filter.id=id
  filter.setAttribute('filterUnits','userSpaceOnUse')
  filter.setAttribute('color-interpolation-filters','sRGB')
  const map=document.createElementNS(svgNS,'feImage')
  map.setAttribute('result','rim');map.setAttribute('preserveAspectRatio','none')
  const displacement=document.createElementNS(svgNS,'feDisplacementMap')
  displacement.setAttribute('in','SourceGraphic');displacement.setAttribute('in2','rim')
  displacement.setAttribute('scale','3')
  displacement.setAttribute('xChannelSelector','R');displacement.setAttribute('yChannelSelector','G')
  filter.append(map,displacement);svg.append(filter);document.body.append(svg)
  let previous='',frame=0
  const draw=()=>{
   const w=Math.round(el.offsetWidth),h=Math.round(el.offsetHeight)
   if(!w||!h)return
   const radius=Math.min(parseFloat(getComputedStyle(el).borderTopLeftRadius)||24,w/2,h/2)
   const key=`${w}:${h}:${radius}`
   if(key===previous)return
   previous=key
   const canvas=document.createElement('canvas');canvas.width=w;canvas.height=h
   const ctx=canvas.getContext('2d')
   if(!ctx)return
   const pixels=ctx.createImageData(w,h),band=Math.min(5,radius)
   for(let y=0;y<h;y++)for(let x=0;x<w;x++){
    const px=x+.5-w/2,py=y+.5-h/2
    const qx=Math.abs(px)-(w/2-radius),qy=Math.abs(py)-(h/2-radius)
    const ax=Math.max(qx,0),ay=Math.max(qy,0),length=Math.hypot(ax,ay)
    const distance=radius-length-Math.min(Math.max(qx,qy),0)
    let nx=0,ny=0
    if(length){nx=ax/length*Math.sign(px);ny=ay/length*Math.sign(py)}
    else if(qx>qy)nx=Math.sign(px);else ny=Math.sign(py)
    const bend=distance>0&&distance<band?Math.sin(Math.PI*distance/band)*.85:0
    const i=(y*w+x)*4
    pixels.data[i]=Math.round(127.5-nx*bend*127)
    pixels.data[i+1]=Math.round(127.5-ny*bend*127)
    pixels.data[i+2]=128;pixels.data[i+3]=255
   }
   ctx.putImageData(pixels,0,0)
   filter.setAttribute('x','0');filter.setAttribute('y','0')
   filter.setAttribute('width',String(w));filter.setAttribute('height',String(h))
   map.setAttribute('width',String(w));map.setAttribute('height',String(h))
   map.setAttribute('href',canvas.toDataURL())
   el.style.setProperty('--glass-filter',`url(#${id})`)
  }
  const observer=new ResizeObserver(()=>{cancelAnimationFrame(frame);frame=requestAnimationFrame(draw)})
  observer.observe(el)
  cleanup.set(el,()=>{observer.disconnect();cancelAnimationFrame(frame);svg.remove();el.style.removeProperty('--glass-filter')})
 },
 unmounted(el){cleanup.get(el)?.();cleanup.delete(el)}
}
