import { Avatar, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
	HoverCard,
	HoverCardContent,
	HoverCardTrigger,
} from "@/components/ui/hover-card";
import { Link } from "react-router-dom";

export function Profile({ name, uid, avatar }) {
	console.log(name, uid, avatar)
	return (
		<div className="flex justify-start ml-16 items-center">
			<Avatar className="h-20 w-20">
				<AvatarImage src={avatar} />
			</Avatar>
			<div className="space-y-1 ml-2">
				<div className="self-center py-2 text-transparent bg-gradient-to-r to-fuchsia-500 from-cyan-500 bg-clip-text w-fit">
					<h1 className="text-3xl font-bold tracking-tighter sm:text-5xl xl:text-6xl/none">
						T5 GameCenter
					</h1>
				</div>
				{
					(uid && uid!="") && <p className="sm:text-2xl">uid:{uid}</p>
				}
			</div>
		</div>
	);
}
