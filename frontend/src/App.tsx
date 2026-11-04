import { useCallback, useEffect, useState } from 'react'
import { api, setUnauthorizedHandler, type Me } from './api'
import LoginPage from './pages/Login'
import MaterialsPage from './pages/Materials'
import AskPage from './pages/Ask'
import TutorPage from './pages/Tutor'
import TeacherPage from './pages/Teacher'
import MCPPage from './pages/MCP'
import AIAppsPage from './pages/AIApps'
import type { AppLaunch } from './ai-apps'
import Icon from './components/Icon'
import { ToastProvider } from './components/Toast'
export type Theme='light'|'dark'
type Page='apps'|'materials'|'ask'|'tutor'|'agents'|'skills'|'mcp'
export default function App(){
 const [me,setMe]=useState<Me|null>(null),[loading,setLoading]=useState(true),[page,setPage]=useState<Page>('tutor'),[jump,setJump]=useState<number|null>(null)
 const [appLaunch,setAppLaunch]=useState<AppLaunch|null>(null)
 const [theme,setTheme]=useState<Theme>(()=>localStorage.getItem('theme')==='dark'?'dark':'light')
 const toggleTheme=()=>setTheme(t=>t==='light'?'dark':'light')
 useEffect(()=>{document.documentElement.dataset.theme=theme;localStorage.setItem('theme',theme)},[theme])
 const expired=useCallback(()=>setMe(null),[])
 useEffect(()=>{setUnauthorizedHandler(expired);window.addEventListener('tutor-unauthorized',expired);return()=>window.removeEventListener('tutor-unauthorized',expired)},[expired])
 function signedIn(u:Me){setAppLaunch(null);setPage(u.role==='teacher'?'agents':'tutor');setJump(null);setMe(u)}
 useEffect(()=>{api.me().then(signedIn).catch(()=>setMe(null)).finally(()=>setLoading(false))},[])
 const openMaterial=useCallback((id:number)=>{setJump(id);setPage('materials')},[])
 async function logout(){await api.logout().catch(()=>{});setMe(null)}
 const navigation=me?.role==='teacher'?[{id:'apps',label:'AI 应用',icon:'grid'},{id:'agents',label:'智能体配置',icon:'agent'},{id:'skills',label:'技能管理',icon:'spark'},{id:'mcp',label:'MCP 工具',icon:'link'},{id:'materials',label:'班级资料',icon:'book'},{id:'ask',label:'知识问答',icon:'search'}]:[{id:'apps',label:'AI 应用',icon:'grid'},{id:'tutor',label:'智能体',icon:'agent'},{id:'materials',label:'资料库',icon:'book'},{id:'ask',label:'知识问答',icon:'search'}]
 return <ToastProvider>{loading?<div className="boot-state"><div className="loading-dot"/>正在进入班级空间…</div>:!me?<LoginPage onSuccess={signedIn} theme={theme} onToggleTheme={toggleTheme}/>:<div className="app-shell"><aside className="app-sidebar"><div><a href="/" className="wordmark">CAMPUSCLAW<span className="brand-dot"/></a><div className="class-caption">{me.class_name}<span>{me.role==='teacher'?'教师工作台':'学生学习空间'}</span></div></div><div className="sidebar-label">工作空间</div><nav aria-label="应用导航">{navigation.map(n=><button key={n.id} className={'nav-item '+(page===n.id?'active':'')} aria-current={page===n.id?'page':undefined} onClick={()=>setPage(n.id as Page)}><span className="nav-icon"><Icon name={n.icon}/></span>{n.label}{page===n.id&&<span className="nav-mark"/>}</button>)}</nav><div className="sidebar-tip"><Icon name={me.role==='teacher'?'spark':'book'} size={24}/><strong>{me.role==='teacher'?'配置好每一份引导':'从你的思路开始'}</strong><p>{me.role==='teacher'?'为本班智能体选择合适的技能与学习资料。':'说说你已经做到哪一步，让助手陪你一起想。'}</p></div><div className="sidebar-account"><span className="avatar">{me.role==='teacher'?'教':'学'}</span><div><strong>{me.username}</strong><small>{me.role==='teacher'?'教师':'学生'} · {me.class_name}</small></div><div className="account-actions"><button className="icon-button" aria-label={theme==='light'?'切换到深色模式':'切换到浅色模式'} onClick={toggleTheme}><Icon name={theme==='light'?'moon':'sun'} size={18}/></button><button className="icon-button" aria-label="登出" onClick={()=>void logout()}><Icon name="logout" size={18}/></button></div></div></aside><div className="workspace-body">{page==='apps'&&<AIAppsPage me={me} onLaunch={v=>{setAppLaunch(v);setPage('tutor')}}/>}{me.role==='student'&&<div hidden={page!=='tutor'}><TutorPage me={me} onOpenMaterial={openMaterial} appLaunch={appLaunch} onLaunchConsumed={()=>setAppLaunch(null)}/></div>}{me.role==='teacher'&&(page==='agents'||page==='skills')&&<TeacherPage me={me} mode={page}/>} {me.role==='teacher'&&page==='mcp'&&<MCPPage me={me}/>} {page==='materials'&&<MaterialsPage me={me} theme={theme} onToggleTheme={toggleTheme} onLogout={logout} onOpenAsk={()=>setPage('ask')} jumpToDetail={jump} onJumpConsumed={()=>setJump(null)}/>} {page==='ask'&&<AskPage me={me} theme={theme} onToggleTheme={toggleTheme} onLogout={logout} onOpenMaterials={()=>setPage('materials')} onOpenMaterial={openMaterial}/>}</div></div>}</ToastProvider>
}
