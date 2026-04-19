import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import MenuView from './pages/MenuView'
import DishDetail from './pages/DishDetail'
import ActionHistory from './pages/ActionHistory'
import Settings from './pages/Settings'
import UberCallback from './pages/UberCallback'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* OAuth callback — no layout wrapper */}
        <Route path="/auth/uber/callback" element={<UberCallback />} />

        {/* Main dashboard */}
        <Route element={<Layout />}>
          <Route index element={<Navigate to="/menu" replace />} />
          <Route path="/menu" element={<MenuView />} />
          <Route path="/menu/:id" element={<DishDetail />} />
          <Route path="/history" element={<ActionHistory />} />
          <Route path="/settings" element={<Settings />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
