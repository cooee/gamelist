import { Avatar, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "@/components/ui/hover-card";
import { Link } from "react-router-dom";

export function Profile({ name, nim, username }) {
	return (
		<div className="flex justify-start ml-16 items-center">
		<Avatar className="h-20 w-20">
			<AvatarImage src={`https://github.com/${username}.png`} />
		</Avatar>
		<div className="space-y-4 ml-2">
			<h4 className="text-sm font-semibold">@{username}</h4>
			<p className="text-sm">{nim}</p>
			<p className="text-sm">{name}</p>
			
		</div>
	</div>
	);
}
