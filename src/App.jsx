import { ThemeProvider } from "@/components/theme-provider";
import { Route, BrowserRouter as Router, Routes } from 'react-router-dom';
import About from "./pages/About";
import Browse from "./pages/Browse";
import Game from "./pages/Game";
import Home from "./pages/Home";
import NotFound from "./pages/NotFound";

const base = process.env.NODE_ENV === "development" ? "/" : "/tets" // 默认 '/'

function App() {
	
	return (
		<ThemeProvider defaultTheme="system" storageKey="vite-ui-theme">
			<Router basename={base}>
				<Routes>
					<Route path="/" element={<Home />} />
					{/* <Route path="/browse" element={<Browse />} />
					<Route path="/browse/:ordering" element={<Browse />} />
					<Route path="/about" element={<About />} />
					<Route path="/game/:id" element={<Game />} /> */}
					<Route path="*" element={<NotFound />} />
				</Routes>
			</Router>
		</ThemeProvider>
	);
}

export default App;
