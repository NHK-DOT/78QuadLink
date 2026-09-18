// A real cross-process snapshot test without ROS or Gazebo.
#include <quadlink78/shared_state_board.hpp>
#include <array>
#include <chrono>
#include <iostream>
#include <sys/wait.h>
#include <cstdlib>
using simulation_state::Board;
int main() {
 char path[]="/tmp/quadlink78-test.XXXXXX";
 int fd=mkstemp(path);if(fd<0)return 1;
 auto magic=Board::magic;
 if(ftruncate(fd,Board::bytes)||pwrite(fd,&magic,sizeof(magic),0)!=sizeof(magic))return 2;
 close(fd);
 pid_t pid=fork();if(pid<0)return 3;
 if(pid==0){
  Board writer;if(!writer.open(path,0))_exit(4);
  Board competitor;if(competitor.open(path,0))_exit(5);
  for(std::uint64_t n=1;n<=10000;n++){
   std::array<std::uint64_t,8> values{};for(auto&v:values)v=n;
   if(!writer.publish(reinterpret_cast<const std::uint8_t*>(values.data()),sizeof(values)))_exit(6);
  }
  usleep(100000);_exit(0);
 }
 Board reader;if(!reader.open(path))return 7;
 std::uint64_t revision=0,last=0;std::size_t size=0;int count=0;
 auto end=std::chrono::steady_clock::now()+std::chrono::seconds(3);
 while(std::chrono::steady_clock::now()<end&&last!=10000){
  std::array<std::uint64_t,8> values{};
  if(reader.snapshot(0,reinterpret_cast<std::uint8_t*>(values.data()),sizeof(values),size,revision)){
   if(size!=sizeof(values))return 8;
   for(auto v:values)if(v!=values[0])return 9;
   if(values[0]<last)return 10;last=values[0];++count;
  }
 }
 int status=0;waitpid(pid,&status,0);unlink(path);
 if(!WIFEXITED(status)||WEXITSTATUS(status)||last!=10000||count==0)return 11;
 std::cout<<"PASS: "<<count<<" consistent snapshots; single writer ownership; final revision "<<revision<<"\n";
 return 0;
}
