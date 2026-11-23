import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import UpdateNotice from './components/UpdateNotice'
import './styles.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <UpdateNotice /><App />
  </React.StrictMode>,
)
