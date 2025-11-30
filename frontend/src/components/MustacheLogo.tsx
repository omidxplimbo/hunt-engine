import { useEffect, useState } from 'react';

const MustacheLogo = () => {
  const [time, setTime] = useState(new Date().toLocaleTimeString());

  useEffect(() => {
    const interval = setInterval(() => {
      setTime(new Date().toLocaleTimeString());
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  // طرح اصلاح شده (سیبیل به سمت چپ شیفت داده شد)
  const asciiArt = `
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-
\`7MMM.     ,MMF'                      mm                        MM
  MMMb    dPMM                        MM                        MM
  M YM   ,M MM \`7MM  \`7MM  ,pP"Ybd mmMMmm  ,6"Yb.    ,p6"bo   MMpMMMb.    .gP"Ya
  M  Mb  M' MM   MM    MM  8I   \`"   MM   8) MM 6M  'OO       MM    MM   ,M'   Yb
  M  YM.P'  MM   MM    MM  \`YMMMa.   MM    ,pm9MM    8M       MM    MM   8M""""""
  M  \`YM'   MM   MM    MM  L.   I8   MM   8M   MM    YM.    , MM    MM   YM.    ,
.JML. \`'  .JMML. \`Mbod"YML.M9mmmP'   \`Mbmo\`Moo9^Yo.  YMbmd' .JMML  JMML.  \`Mbmmd'

        ...:~!!~:..........^~~^....^~~^..........:~!!~:...
        .!PBGP5PG5^.....:?PB##BGJJPB##BP?:.....^YGP5PGBP!.
        J#B!:...^PG...:?B################B?:...GP^...:!B#J
        ##7 .....!7..7G####################G7: 7!..... !##
        P&Y.......^?G###########YY###########G?^.......Y&P
        ^5#GJ7!7JPB##########BY~..~YB##########BPJ7!7JG#5^
        ..~YG##&&&&####BBG5?!:......:!?5GBB####&&&&##GY!..
        ....:^~!7777!!~^:................:^~!!7777!~^:....
`;

  return (
    <div className="flex flex-col items-center justify-center w-full overflow-hidden py-2">
      {/* ASCII Art */}
      <pre className="font-mono text-red-500 text-[4px] leading-[4px] font-bold whitespace-pre text-center select-none opacity-90 tracking-tighter">
        {asciiArt}
      </pre>
      
      <div className="mt-4 text-center space-y-1 border-t border-red-900/30 w-full pt-2">
        <p className="text-[10px] font-bold text-red-400 uppercase tracking-widest">
          Mustache Security
        </p>
        <p className="text-[9px] text-red-500/60 font-mono">
          Researcher Team
        </p>
        <p className="text-[10px] text-gray-500 font-mono mt-1 bg-gray-900/50 rounded py-0.5">
          🕒 {time}
        </p>
      </div>
    </div>
  );
};

export default MustacheLogo;