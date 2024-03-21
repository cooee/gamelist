import NavBar from "@/components/NavBar";
import { Button } from "@/components/ui/button";
import { ChevronRight } from "lucide-react";
import { useEffect, useState, useCallback } from "react";
import { Link } from "react-router-dom";
import GameCard from "../components/GameCard";
import { GameCardSkeleton } from "../components/GameCardSkeleton";
import { Profile } from "../components/Profile"
import { PopularGames } from "../config/constants";
import { useTheme } from "@/components/theme-provider";
import { access_token, getAuthCode, user_profile, getQuerys } from "../lib/GateSDK";



export default function Home() {
  const size = 5;
  const [popularGames, setPopularGames] = useState([]);
  const [topGames, setTopGames] = useState([]);
  const [newGames, setNewGames] = useState([]);
  const [profile, setProfile] = useState({ name: "", uid: "", avatar: "" });
  const { setTheme } = useTheme();
  const [authurl, setAuthurl] = useState("");
  const [msg, setDebugMsg] = useState("");

  const requestAuthCode = useCallback(() => {
    const code = getQuerys(window.location.href)["code"] || "";
    if (code && code != "") {
      access_token(code).then((response) => {
        console.log(response.data);
        if (response.data.error && response.data.error != "") {
          console.log(response.data.toString());
        } else {
          const tokenData = response.data
          tokenData.expired_at = tokenData.expired_in + Math.floor(Date.now() / 1000)
          const access_token_data = JSON.stringify(tokenData)
          localStorage.setItem("access_token", access_token_data)
          console.log(access_token_data);
          doLogin()
        }
      })
        .catch((error) => {
          console.log(JSON.stringify(error));
          localStorage.setItem("access_token", "");
        });
    } else {
      console.log("requestAuthCode");
      getAuthCode().then((response) => {
        console.log(response.data);
        setAuthurl(response.data.toString())
      })
        .catch((error) => {
          console.log(error);
          console.log(JSON.stringify(error));
        });;
    }
  }, []);

  const doLogin = useCallback(() => {
    const access_token_data = localStorage.getItem("access_token")
    setDebugMsg(access_token_data)
    if (access_token_data && access_token_data != "") {
      const token = JSON.parse(access_token_data)
      const now = Math.floor(Date.now() / 1000)
      console.log(token)
      // 过期了
      if (now > token.expired_at) {
        requestAuthCode()

      } else {
        const access_code = token.access_code;
        user_profile(access_code).then((response) => {
          console.log(response.data);
          const resp = response.data
          setProfile({
            name: resp.data.name,
            uid: resp.data.uid,
            avatar: resp.data.avatar
          })
        })
      }

    } else {
      requestAuthCode()
    }
  }, []);


  useEffect(() => {

    setTheme("dark")

    const fetchPopularGames = async () => {
      try {
        // const response = await fetch(
        //   `https://api.rawg.io/api/games?key=${
        //     import.meta.env.VITE_RAWG_API_KEY
        //   }&page_size=${size}&ordering=popularity`
        // );
        // const data = await response.json();
        // console.log("fetchPopularGames", data.results);
        setPopularGames(PopularGames);
      } catch (error) {
        console.error("Error fetching game: ", error);
      }
    };
    const fetchTopGames = async () => {
      try {
        const response = await fetch(
          `https://api.rawg.io/api/games?key=${import.meta.env.VITE_RAWG_API_KEY
          }&page_size=${size}&ordering=-metacritic`
        );
        const data = await response.json();
        setTopGames(data.results);
      } catch (error) {
        console.error("Error fetching game: ", error);
      }
    };
    const fetchNewGames = async () => {
      try {
        const response = await fetch(
          `https://api.rawg.io/api/games?key=${import.meta.env.VITE_RAWG_API_KEY
          }&page_size=${size}&ordering=released`
        );
        const data = await response.json();
        setNewGames(data.results);
      } catch (error) {
        console.error("Error fetching game: ", error);
      }
    };

    fetchPopularGames();




    // fetchTopGames();
    // fetchNewGames();
  }, []);

  useEffect(() => {
    console.log("useEffect do login");
    doLogin();
  }, []);




  return (
    <>
      <div >
        <iframe srcDoc={authurl} width="1"
          height="1"></iframe>
      </div>
      {/* <NavBar /> */}
      <div className="py-8 sm:container md:py-16">

        <Profile name={profile.name} avatar={profile.avatar} uid={profile.uid}></Profile>
        <div className="px-8 md:px-16">
          <section className="w-full py-16">
            <div className="grid gap-6 lg:grid-cols-[1fr_400px] lg:gap-12 xl:grid-cols-[1fr_600px]">
              <div className="flex flex-col justify-center space-y-4">
                <div className="space-y-2">
                  <p className="max-w-[600px] text-gray-500 md:text-xl dark:text-gray-400">
                    Discover your next favorite game from T5 GameCenter. Start
                    your gaming journey now!
                  </p>
                </div>
                <div className="flex flex-col gap-2 min-[400px]:flex-row">
                </div>
              </div>
            </div>
          </section>
          <div className="mt-8">
            <div className="flex flex-col items-start gap-4 md:flex-row md:items-center md:gap-12">
              <div className="grid gap-1 mb-4">
                <h1 className="text-xl font-bold">Popular Games</h1>
                <p className="text-foreground/60">
                  Discover the most popular games.
                </p>
              </div>
              <Link to="/browse/popularity" className="shrink-0 md:ml-auto ">
                <Button size="lg" variant="outline">
                  View All
                </Button>
              </Link>
            </div>
            <div className="grid grid-cols-2 grid-rows-1 gap-4 md:grid-cols-4 lg:grid-cols-5 mt-2">
              {popularGames.map((game) => (
                <GameCard key={game.id} game={game} view="grid" />
              ))}
              {!popularGames.length && (
                <>
                  {Array.from({ length: size }).map((_, index) => (
                    <GameCardSkeleton key={index} view="grid" />
                  ))}
                </>
              )}
            </div>
          </div>

          {/* <div className="mt-8">
            <div className="flex flex-col items-start gap-4 md:flex-row md:items-center md:gap-8">
              <div className="grid gap-1 mb-4">
                <h1 className="text-xl font-bold">Top Rated Games</h1>
                <p className="text-foreground/60">
                  Discover the highest rated games.
                </p>
              </div>
              <Link to="/browse/-metacritic" className="shrink-0 md:ml-auto">
                <Button size="lg" variant="outline">
                  View All
                </Button>
              </Link>
            </div>
            <div className="grid grid-cols-2 grid-rows-1 gap-4 md:grid-cols-4 lg:grid-cols-5">
              {topGames.map((game) => (
                <GameCard key={game.id} game={game} view="grid" />
              ))}
              {!topGames.length && (
                <>
                  {Array.from({ length: size }).map((_, index) => (
                    <GameCardSkeleton key={index} view="grid" />
                  ))}
                </>
              )}
            </div>
          </div> */}

          {/* <div className="mt-8">
            <div className="flex flex-col items-start gap-4 md:flex-row md:items-center md:gap-8">
              <div className="grid gap-1 mb-4">
                <h1 className="text-xl font-bold">New Games</h1>
                <p className="text-foreground/60">
                  Discover the latest released games.
                </p>
              </div>
              <Link to="/browse/released" className="shrink-0 md:ml-auto">
                <Button size="lg" variant="outline">
                  View All
                </Button>
              </Link>
            </div>
            <div className="grid grid-cols-2 grid-rows-1 gap-4 md:grid-cols-4 lg:grid-cols-5">
              {newGames.map((game) => (
                <GameCard key={game.id} game={game} view="grid" />
              ))}
              {!newGames.length && (
                <>
                  {Array.from({ length: size }).map((_, index) => (
                    <GameCardSkeleton key={index} view="grid" />
                  ))}
                </>
              )}
            </div>
          </div> */}
        </div>
      </div>
    </>
  );
}