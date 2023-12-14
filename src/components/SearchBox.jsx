import { Button } from "@/components/ui/button";
import {
	CommandDialog,
	CommandEmpty,
	CommandGroup,
	CommandItem,
	CommandList,
} from "@/components/ui/command";
import { Input } from "@/components/ui/input";
import { Search } from "lucide-react";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Image from "./Image";

export default function SearchBox() {
	const [open, setOpen] = useState(false);
	const [searchQuery, setSearchQuery] = useState("");
	const [searchResults, setSearchResults] = useState([]);

	useEffect(() => {
		const down = (e) => {
			if (e.key === "s" && (e.metaKey || e.ctrlKey)) {
				e.preventDefault();
				setOpen((open) => !open);
			}
		};
		document.addEventListener("keydown", down);
		return () => document.removeEventListener("keydown", down);
	}, []);

	useEffect(() => {
		// Fetch data based on searchQuery and update searchResults
		const fetchData = async () => {
			try {
				const response = await fetch(
					`https://api.rawg.io/api/games?key=dc6f3f19206d43078b51b87ab10705b1&search=${searchQuery}&page_size=10`
				);
				const data = await response.json();
				// console.log(data.results);
				setSearchResults(data.results || []);
			} catch (error) {
				console.error("Error fetching data: ", error);
			}
		};

		fetchData();
	}, [searchQuery]);

	const handleButtonClick = () => {
		setOpen(true);
	};

	return (
		<>
			<div className="w-full flex-1 md:w-auto md:flex-none">
				<Button
					variant="outline"
					className="w-full md:w-[200px] justify-between"
					onClick={handleButtonClick}
				>
					Search games...
					<p className="text-sm text-muted-foreground">
						<kbd className="pointer-events-none inline-flex h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground opacity-100">
							<span className="text-xs">⌘</span>S
						</kbd>
					</p>
				</Button>
			</div>

			<CommandDialog className="rounded-lg" open={open} onOpenChange={setOpen}>
				<div className="flex items-center border-b px-3">
					<Search className="mr-2 h-4 w-4 shrink-0 opacity-50" />
					<Input
						placeholder="Search games"
						className={
							"flex h-11 w-full rounded-md bg-transparent py-3 text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 border-none focus-visible:outline-none focus-visible:ring-0 focus-visible:ring-offset-0"
						}
						onChange={(e) => {
							setSearchQuery(e.target.value);
						}}
					/>
				</div>
				<CommandList>
					<CommandEmpty>No results found.</CommandEmpty>
					<CommandGroup heading="Results">
						{searchResults.map((result) => (
							<Link to={`/game/${result.id}`} key={result.id}>
								<CommandItem
									className="cursor-pointer"
									onPointerDown={() => {
										setOpen(false);
									}}
								>
									<div className="flex gap-2 items-center">
										<div>
											{result.background_image ? (
												<Image
													src={result.background_image}
													alt={result.name}
													className="aspect-[1.5/1] object-cover w-20 rounded"
												/>
											) : (
												<div className="aspect-[1.5/1] object-cover w-20 rounded bg-secondary flex items-center text-center justify-center overflow-hidden">
													<span className="text-xs">{result.name}</span>
												</div>
											)}
										</div>
										{result.name}
									</div>
								</CommandItem>
							</Link>
						))}
					</CommandGroup>
				</CommandList>
			</CommandDialog>
		</>
	);
}
